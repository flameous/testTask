package testTask

import (
	"sync"
	"fmt"
	"errors"
	"time"
)

var (
	cacheMap map[string]*Cache
	mtxCache sync.Mutex

	queue    Queue
	mtxQueue sync.Mutex

	//todo: ttl инфы в кэше - 30 секунд
	validTimeSeconds int64 = 30
)

type CacheMap map[string]*Cache

type Cache struct {
	data         *interface{}
	creationTime int64
}

type Queue struct {
	qMap map[string]*SomeStruct

	q []string
}

type SomeStruct struct {
	getter    func() (interface{}, error)
	Receivers []chan Result
}

type Result struct {
	i   interface{}
	err error
}

// если есть в кэше, то сразу отдаём инфу
func checkCache(key string) (*interface{}, bool) {
	defer mtxCache.Unlock()
	mtxCache.Lock()

	if d, ok := cacheMap[key]; ok {
		return d.data, ok
	}
	return nil, false
}

func eraseCache() {
	defer mtxCache.Unlock()
	mtxCache.Lock()

	tn := time.Now().Unix()
	for k, v := range cacheMap {
		if tn-v.creationTime > validTimeSeconds {
			delete(cacheMap, k)
		}
	}
}

func writeToCache(key string, i interface{}) {
	defer mtxCache.Unlock()
	mtxCache.Lock()

	cacheMap[key] = &Cache{
		&i,
		time.Now().Unix(),
	}
}

func Get(key string, getter func() (interface{}, error)) (i interface{}, err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = errors.New(fmt.Sprintf("unhandled error: %v", r))
		}
	}()

	d, ok := checkCache(key)
	if ok {
		i = *d
		return
	}

	// геттер становится в очередь к БД
	chGetter := make(chan Result)
	defer close(chGetter)

	addToQueue(key, getter, chGetter)

	result := <-chGetter
	i = result.i
	err = result.err

	if err == nil {
		writeToCache(key, i)
	}
	return
}

func addToQueue(key string, getter func() (interface{}, error), result chan Result) {
	defer mtxQueue.Unlock()
	mtxQueue.Lock()

	// если геттер по такому ключу уже в очереди, то записываем наш канал в получателях
	if v, ok := queue.qMap[key]; ok {
		v.Receivers = append(v.Receivers, result)
		return
	}

	// если же мы первые, то создаём такую пару k/v и добавляем наш канал в получатели
	queue.qMap[key] = &SomeStruct{
		getter,
		[]chan Result{result},
	}

	queue.q = append(queue.q, key)
}

func QueueLifecycle() {
	for {
		if len(queue.q) != 0 {

			// ключ первого в очереди геттера
			key := queue.q[0]
			if key == "" {
				continue
			}

			task, ok := queue.qMap[key]

			// такое, _вроде бы_, никогда не может случиться.
			if ! ok {
				fmt.Println(`key is`, key)
				panic(fmt.Sprintf("Несуществующий ключ в очереди. "+
					"\"%s\". queue.qMap: %v, queue.q: %v", key, queue.qMap, queue.q))
			}

			i, err := task.getter()
			r := Result{i, err}

			for _, v := range task.Receivers {
				v <- r
			}

			func() {
				defer mtxQueue.Unlock()
				mtxQueue.Lock()

				// удаляем наш ключ из мапы геттеров
				delete(queue.qMap, key)

				// переходим к следующему запросу
				queue.q = queue.q[1:]
			}()
		}
	}
}

func init() {
	cacheMap = make(map[string]*Cache)

	queue = Queue{
		qMap: make(map[string]*SomeStruct),
		q:    make([]string, 1),
	}

	go QueueLifecycle()

	go func(timeout time.Duration) {
		working := true
		for working {
			select {
			case <-time.After(timeout):
				working = false


			case <-time.After(100 * time.Millisecond):
				eraseCache()
			}
		}
	}(5 * time.Minute)
}
