package tunny

import (
    "sync"
    "testing"
)

/*--------------------------------------------------------------------------------------------------
 */

func TestBasicJob(t *testing.T) {
    pool, err := CreatePool(1, func(in interface{}) interface{} {
        intVal := in.(int)
        return intVal * 2
    }).Open()
    if err != nil {
        t.Errorf("Failed to create pool: %v", err)
        return
    }
    defer pool.Close()

    for i := 0; i < 1; i++ {
        ret, err := pool.SendWork(10)
        if err != nil {
            t.Errorf("Failed to send work: %v", err)
            return
        }
        retInt := ret.(int)
        if ret != 20 {
            t.Errorf("Wrong return value: %v != %v", 20, retInt)
        }
    }
}

func TestParallelJobs(t *testing.T) {
    nWorkers := 10

    jobGroup := sync.WaitGroup{}
    testGroup := sync.WaitGroup{}

    pool, err := CreatePool(nWorkers, func(in interface{}) interface{} {
        jobGroup.Done()
        jobGroup.Wait()

        intVal := in.(int)
        return intVal * 2
    }).Open()
    if err != nil {
        t.Errorf("Failed to create pool: %v", err)
        return
    }
    defer pool.Close()

    for j := 0; j < 1; j++ {
        jobGroup.Add(nWorkers)
        testGroup.Add(nWorkers)

        for i := 0; i < nWorkers; i++ {
            go func() {
                ret, err := pool.SendWork(10)
                if err != nil {
                    t.Errorf("Failed to send work: %v", err)
                    return
                }
                retInt := ret.(int)
                if ret != 20 {
                    t.Errorf("Wrong return value: %v != %v", 20, retInt)
                }

                testGroup.Done()
            }()
        }

        testGroup.Wait()
    }
}

/*--------------------------------------------------------------------------------------------------
 */

// Basic worker implementation
type dummyWorker struct {
    ready bool
    t     *testing.T
}

func (d *dummyWorker) TunnyJob(in interface{}) interface{} {
    if !d.ready {
        d.t.Errorf("TunnyJob called without polling TunnyReady")
    }
    d.ready = false
    return in
}

func (d *dummyWorker) TunnyReady() bool {
    d.ready = true
    return d.ready
}

// Test the pool with a basic worker implementation
func TestDummyWorker(t *testing.T) {
    pool, err := CreateCustomPool([]TunnyWorker{&dummyWorker{t: t}}).Open()
    if err != nil {
        t.Errorf("Failed to create pool: %v", err)
        return
    }
    defer pool.Close()

    for i := 0; i < 100; i++ {
        if result, err := pool.SendWork(12); err != nil {
            t.Errorf("Failed to send work: %v", err)
        } else if resInt, ok := result.(int); !ok || resInt != 12 {
            t.Errorf("Unexpected result from job: %v != %v", 12, result)
        }
    }
}

// Extended worker implementation
type dummyExtWorker struct {
    dummyWorker

    initialized bool
}

func (d *dummyExtWorker) TunnyJob(in interface{}) interface{} {
    if !d.initialized {
        d.t.Errorf("TunnyJob called without calling TunnyInitialize")
    }
    return d.dummyWorker.TunnyJob(in)
}

func (d *dummyExtWorker) TunnyInitialize() {
    d.initialized = true
}

func (d *dummyExtWorker) TunnyTerminate() {
    if !d.initialized {
        d.t.Errorf("TunnyTerminate called without calling TunnyInitialize")
    }
    d.initialized = false
}

// Test the pool with an extended worker implementation
func TestDummyExtWorker(t *testing.T) {
    pool, err := CreateCustomPool(
    []TunnyWorker{
        &dummyExtWorker{
            dummyWorker: dummyWorker{t: t},
        },
    }).Open()
    if err != nil {
        t.Errorf("Failed to create pool: %v", err)
        return
    }
    defer pool.Close()

    for i := 0; i < 100; i++ {
        if result, err := pool.SendWork(12); err != nil {
            t.Errorf("Failed to send work: %v", err)
        } else if resInt, ok := result.(int); !ok || resInt != 12 {
            t.Errorf("Unexpected result from job: %v != %v", 12, result)
        }
    }
}

// Extended and interruptible worker implementation
type dummyExtIntWorker struct {
    dummyExtWorker

    jobLock *sync.Mutex
}

func (d *dummyExtIntWorker) TunnyJob(in interface{}) interface{} {
    d.jobLock.Lock()
    d.jobLock.Unlock()

    return d.dummyExtWorker.TunnyJob(in)
}

func (d *dummyExtIntWorker) TunnyReady() bool {
    d.jobLock.Lock()

    return d.dummyExtWorker.TunnyReady()
}

func (d *dummyExtIntWorker) TunnyInterrupt() {
    d.jobLock.Unlock()
}

// Test the pool with an extended and interruptible worker implementation
func TestDummyExtIntWorker(t *testing.T) {
    pool, err := CreateCustomPool(
    []TunnyWorker{
        &dummyExtIntWorker{
            dummyExtWorker: dummyExtWorker{
                dummyWorker: dummyWorker{t: t},
            },
            jobLock: &sync.Mutex{},
        },
    }).Open()
    if err != nil {
        t.Errorf("Failed to create pool: %v", err)
        return
    }
    defer pool.Close()

    for i := 0; i < 100; i++ {
        if _, err := pool.SendWorkTimed(1, nil); err == nil {
            t.Errorf("Expected timeout from dummyExtIntWorker.")
        }
    }
}
