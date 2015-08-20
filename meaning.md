# 线程池

- 线程池是一种多线程处理形式，处理过程中将任务添加到队列，然后在创建线程后自动启动这些任务。
- 线程池线程都是后台线程。每个线程都使用默认的堆栈大小，以默认的优先级运行，并处于多线程单元中。
- 如果某个线程在托管代码中空闲（如正在等待某个事件）,则线程池将插入另一个辅助线程来使所有处理器保持繁忙。
- 如果所有线程池线程都始终保持繁忙，但队列中包含挂起的工作，则线程池将在一段时间后创建另一个辅助线程但线程的数目永远不会超过最大值。
- 超过最大值的线程可以排队，但他们要等到其他线程完成后才启动。

## 组成部分

- 1、线程池管理器（ThreadPoolManager）:用于创建并管理线程池
- 2、工作线程（WorkThread）: 线程池中线程
- 3、任务接口（Task）:每个任务必须实现的接口，以供工作线程调度任务的执行。
- 4、任务队列:用于存放没有处理的任务。提供一种缓冲机制。

### 如何利用已有对象来服务就是一个需要解决的关键问题，其实这就是一些"池化资源"技术产生的原因。
### 创建和销毁对象是很费时间的，因为创建一个对象要获取内存资源或者其它更多资源

## 何时不使用线程池线程：

- 如果需要使一个任务具有特定优先级
- 如果具有可能会长时间运行（并因此阻塞其他任务）的任务
- 如果需要将线程放置到单线程单元中（线程池中的线程均处于多线程单元中）
- 如果需要永久标识来标识和控制线程，比如想使用专用线程来终止该线程，将其挂起或按名称发现它