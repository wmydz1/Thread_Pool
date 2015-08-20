package tunny
import (
    "time"
    "sync/atomic"
)

type workerWrapper struct {
    readyChan chan int
    jobChan chan interface{}
    outputChan chan interface{}
    poolOpen uint32
    worker TunnyWorker
}

func (wrapper *workerWrapper) Loop() {
    tout := time.Duration(5)
    // 我们不能简单检查 job 的channel 是否关闭
    for !wrapper.worker.TunnyReady() {
        if atomic.LoadUint32(&wrapper.poolOpen) == 0 {
            break
        }
        time.Sleep(tout * time.Millisecond)
    }

    wrapper.readyChan <- 1

    for data := range wrapper.jobChan {
        wrapper.outputChan <- wrapper.worker.TunnyJob(data)
        for !wrapper.worker.TunnyReady() {
            if atomic.LoadUint32(&wrapper.poolOpen) == 0 {
                break
            }
            time.Sleep(tout * time.Millisecond)
        }
        wrapper.readyChan <- 1
    }


    close(wrapper.readyChan)
    close(wrapper.outputChan)
}
func (wrapper *workerWrapper) Open() {
    if extWorker, ok := wrapper.worker.(TunnyExtendedWorker); ok {
        extWorker.TunnyInitialize()
    }
    wrapper.readyChan = make(chan int)
    wrapper.jobChan =make(chan interface{})
    wrapper.outputChan =make(chan interface{})

    atomic.SwapUint32(&wrapper.poolOpen, uint32(1))

    go wrapper.Loop()

}
func (wrapper *workerWrapper) Close() {
    close(wrapper.jobChan)

    // 中断没有准备好的 worker
    atomic.SwapUint32(&wrapper.poolOpen, uint32(0))
}

// 遵循 Join ,否则在 worker上不能终止调用
func (wrapper *workerWrapper) Join() {
    // 确认 ready 和 output 两个channel 都被关闭了
    for {
        _, readyOpen := <-wrapper.readyChan
        _, outputOpen := <-wrapper.outputChan
        if !readyOpen && !outputOpen {
            break
        }
    }

    if ewtWorker, ok := wrapper.worker.(TunnyExtendedWorker); ok {
        ewtWorker.TunnyTerminate()
    }
}
// 中断的方法
func (wrapper *workerWrapper) Interrupt() {
    if ewtWorker, ok := wrapper.worker.(TunnyInterruptable); ok {
        ewtWorker.TunnyInterrupt()
    }
}