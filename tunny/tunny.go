package tunny
import (
    "errors"
    "reflect"
    "sync"
    "sync/atomic"

    "time"

)

//这个包实现了一个简单的池来操纵独立的 worker goroutines

var (
    ErrPoolAlreadyRunning = errors.New("the pool is already running")
    ErrPoolNotRunning = errors.New("the pool is not running")
    ErrJobNotFunc = errors.New("generic worker not given a func()")
    ErrWorkerClosed = errors.New("worker was closed")
    ErrJobTimeOut = errors.New("job request timed out")
)

// worker 要实现的接口
type TunnyWorker interface {

    // 调用每一个 job ,除非它已经被同步了
    TunnyJob(interface{}) interface{}

    // 调用每个 job 之后，表明这个 worker 是否准备好下一个 job
    //默认返回 true,如果返回 false, 那么方法每 5 毫秒调用一次 直到返回true或者线程池关闭
    TunnyReady() bool
}
/*
扩展的 worker ,一个可选的接口，需要控制更多的状态可以实现它
*/
type TunnyExtendedWorker interface {
    // 线程池开启的时候调用,在任何 job 被送去执行执行前调用它
    TunnyInitialize()
    // 线程池关闭时候调用，所有任务完成的时候调用
    TunnyTerminate()
}
/*
可中断的 一个可选的接口，实现它可以允许 worker 被遗弃时可以丢到任务
*/

type TunnyInterruptable interface {
    // 当当前的 job 被客户端遗弃时调用
    TunnyInterrupt()
}
/*
默认的非常基本的worker 含有一个闭包 这个闭包是用于调用每个 job的
*/

type tunnyDefaultWorker struct {
    job *func(interface{}) interface{}
}
// fn := *worker.job
// fn(data)
func (worker *tunnyDefaultWorker) TunnyJob(data interface{}) interface{} {
    return (*worker.job)(data)
}
func (worker *tunnyDefaultWorker)  TunnyReady() bool {
    return true
}

/*
工作池里包含的结构体和方法要求和池进行沟通，必须在发送 job 之前打开，job 结束时关闭
你可能要开启关闭池很多次，调用是一种阻塞调用，保证全部的goroutines都停止
*/
type WorkPool struct {
    workers []*workerWrapper
    selects []reflect.SelectCase
    statusMutex sync.RWMutex
    running uint32
}
func (pool *WorkPool) isRunning() bool {
    return (atomic.LoadUint32(&pool.running) == 1)
}
func (pool *WorkPool) setRunning(running bool) {
    if running {
        atomic.SwapUint32(&pool.running, 1)
    }else {
        atomic.SwapUint32(&pool.running, 0)
    }
}
/*
打开被线程池控制 全部的 channel 和启动 后台的 goroutines
*/

func (pool *WorkPool) Open() (*WorkPool, error) {
    pool.statusMutex.Lock()
    defer pool.statusMutex.Unlock()

    if !pool.isRunning() {
        pool.selects =make([]reflect.SelectCase, len(pool.workers))

        for i, workerWrapper := range pool.workers {
            workerWrapper.Open()

            pool.selects[i] = reflect.SelectCase{
                Dir:reflect.SelectRecv,
                Chan: reflect.ValueOf(workerWrapper.readyChan),
            }
        }

        pool.setRunning(true)

        return pool, nil
    }
    return nil, ErrPoolAlreadyRunning
}

/*
关闭被线程池控制 全部的 channel 和启动 后台的 goroutines
*/

func (pool *WorkPool) Close() error {
    pool.statusMutex.Lock()
    defer pool.statusMutex.Unlock()

    if pool.isRunning() {
        for _, workerWrapper := range pool.workers {
            workerWrapper.Close()
        }
        for _, workerWrapper := range pool.workers {
            workerWrapper.Join()
        }
        pool.setRunning(false)
        return nil
    }
    return ErrPoolNotRunning
}
/*
创建一个线程池，每个 job 执行的闭包函数
*/
func CreatePool(numWorkers int, job func(interface{}) interface{}) *WorkPool {
    pool := WorkPool{running:0}

    pool.workers =make([]*workerWrapper, numWorkers)
    for i := range pool.workers {
        newWorker := workerWrapper{
            worker:&(tunnyDefaultWorker{&job}),
        }
        pool.workers[i] =&newWorker
    }
    return &pool
}



func CreatePoolGeneric(numWorkers int) *WorkPool {

    return CreatePool(numWorkers, func(jobCall interface{}) interface{} {
        if method, ok := jobCall.(func()); ok {
            method()
            return nil
        }
        return ErrJobNotFunc
    })

}
/*
创建一个自定义的线程池 - 含有自定义 worker 的数组的线程池
自定义的 worker 必须实现 TunnyWorker 接口 可以选择实现
TunnyExtendedWorker 和 TunnyInterruptable  接口
*/

func CreateCustomPool(customWorkers []TunnyWorker) *WorkPool {
    pool := WorkPool{running:0}

    pool.workers =make([]*workerWrapper, len(customWorkers))
    for i := range pool.workers {
        newWorker := workerWrapper{
            worker:customWorkers[i],
        }
        pool.workers[i] =&newWorker
    }
    return &pool
}
/*
 发送超时 - 发送一个 job 和返回一个结果 这是一个超时的同步调用
*/
func (pool *WorkPool) SendWorkTimed(milliTimeout time.Duration, jobData interface{}) (interface{}, error) {
    pool.statusMutex.RLock()
    defer pool.statusMutex.RUnlock()

    if pool.isRunning() {
        before := time.Now()

        selectCases := append(pool.selects[:], reflect.SelectCase{
            Dir :reflect.SelectRecv,
            Chan: reflect.ValueOf(time.After(milliTimeout * time.Millisecond)),
        })

        // 等待 worker ,或者超时
        if chosen, _, ok := reflect.Select(selectCases); ok {
            // 检查 select 的下标是否是 worker ，否则我们超时
            if chosen <(len(selectCases) -1 ) {
                pool.workers[chosen].jobChan <- jobData

                // 等待响应 否则 超时
                select {
                case data, open := <-pool.workers[chosen].outputChan:
                    if !open {
                        return nil, ErrWorkerClosed
                    }
                    return data, nil
                case <-time.After((milliTimeout * time.Millisecond) - time.Since(before)):
                // 如果这里超时，我们也需要收集 output ,我们可以 把 等待的进程放进一个新的 goroutine.
                    go func() {
                        pool.workers[chosen].Interrupt()
                        <-pool.workers[chosen].outputChan
                    }()
                    return nil, ErrJobTimeOut

                }
            }else {
                return nil, ErrJobTimeOut
            }

        }else {
            // 意味着 选择的 channel 被关闭了
            return nil, ErrWorkerClosed
        }
    }else {
        return nil, ErrPoolNotRunning
    }
}
/*
SendWorkTimedAsync - 发送一个定时的 job
发送结果给 closure，如果将来没有要求，可以把 closure 设置为空
*/
func (pool *WorkPool) SendWorkTimedAsync(
milliTimeout time.Duration,
jobData interface{},
after func(interface{}, error),
) {
    go func() {
        result, err := pool.SendWorkTimed(milliTimeout, jobData)
        if after !=nil {
            after(result, err)
        }
    }()
}
/*
SendWork - 无阻塞给worker发送job 返回一个结果，这是一个同步的调用
*/

func (pool *WorkPool) SendWork(jobData interface{}) (interface{}, error) {
    pool.statusMutex.RLock()
    defer pool.statusMutex.RUnlock()

    if pool.isRunning() {
        if chosen, _, ok := reflect.Select(pool.selects); ok && chosen >=0 {
            pool.workers[chosen].jobChan <- jobData
            result, open := <-pool.workers[chosen].outputChan

            if !open {
                return nil, ErrWorkerClosed
            }
            return result, nil
        }
        return nil, ErrWorkerClosed
    }
    return nil, ErrPoolNotRunning
}
/*
SendWorkAsync - 无阻塞发送 job 给worker
发送结果给 closure，如果将来没有要求，可以把 closure 设置为空
*/
func (pool *WorkPool) SendWorkAsync(jobData interface{}, after func(interface{}, error)) {
    go func() {
        result, err := pool.SendWork(jobData)
        if after!=nil {
            after(result, err)
        }
    }()
}
