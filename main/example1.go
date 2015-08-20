package main
import (
    "sync"
    "runtime"
    "time"
    "io/ioutil"
    "os"
    "fmt"
)

type JobFunc func(int, interface{}, chan interface{})
// 主线程 产生bool的channel
func threadMain(id int, queue chan interface{}, wg *sync.WaitGroup, job JobFunc) chan bool {
    quitCommand := make(chan bool, 1)

    go func() {
        for {
            select {
            case task := <-queue:
                wg.Add(1)
                job(id, task, queue)
                wg.Done()
            case <-quitCommand:
                return
            }
        }
    }()
    return quitCommand
}
// 并发的协程
func Concurrent(queue chan interface{}, job JobFunc) {
    var wg sync.WaitGroup
    cpuCount := runtime.NumCPU()
    // 使用机器最大核数
    runtime.GOMAXPROCS(cpuCount)
     // 创建bool类型的channel数组,容量为cpu核数
    quitCommands := make([]chan bool, cpuCount)
    for i := 0; i<cpuCount; i++ {
        // 往channel数组里面添加数据
        quitCommands[i]= threadMain(i+1, queue, &wg, job)
    }
    // 定时器  10s
    ticker := time.Tick(time.Millisecond *10)
    for _ = range ticker {
        // 如果是空的channel,创建一个bool 类型channel,并且往里面填数据
        if len(queue) ==0 {
            for _, quitCommand := range quitCommands {
                quitCommand <- true
            }
            wg.Wait()
            break
        }
    }
}

func main() {
    // []os.FileInfo, error
    fileInfos, _ := ioutil.ReadDir("/Users/samchen/Applications/Go/src/github.com/logoocc")
    // 创建一个channel存放 os.FileInfo
    queue := make(chan interface{}, len(fileInfos))

    for _, fileInfo := range fileInfos {
        queue <- fileInfo
    }

    Concurrent(queue, func(id int, task interface{}, queue chan interface{}) {
        // 将task interface{} 强制转换为 os.FileInfo
        fileInfo := task.(os.FileInfo)
        fmt.Printf(">> thread %d: %s\n", id, fileInfo.Name())
        // do some work to use fileInfo
        fmt.Printf("<< thread %d\n", id)
    })
}