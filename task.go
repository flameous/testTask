package testTask

import (
	"sync"
	"fmt"
	"errors"
	"time"
	"log"
)

var (
	cacheMap map[string]*Cache
	mtxCache sync.Mutex

	queue    Queue
	mtxQueue sync.Mutex

	//todo: ttl инфы в кэше
	validTimeSeconds int64 = 3

	//todo: обновление кэша нет
)

type Cache struct {
	data         *interface{}
	creationTime int64
}

// Очередь сделана в виде слайса строк-ключей для удобной итерации
// и мапа для хранения всяких нужных вещей
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

func updateCache(key string, i interface{}) {
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
		err = errors.New("from cache!: " + key)
		return
	}

	// геттер становится в очередь к БД
	chGetter := make(chan Result)
	defer close(chGetter)

	addToQueue(key, getter, chGetter)

	result := <-chGetter
	i = result.i
	err = result.err

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
			key := func() string {
				defer mtxQueue.Unlock()
				mtxQueue.Lock()
				return queue.q[0]
			}()

			task, ok := func() (i *SomeStruct, ok bool) {
				defer mtxQueue.Unlock()
				mtxQueue.Lock()

				i, ok = queue.qMap[key]
				return
			}()

			// такое, _вроде бы_, никогда не может случиться.
			if ! ok {
				log.Panicf("Несуществующий ключ в очереди. "+
					"\"%s\". queue.qMap: %v, queue.q: %v.",
					key, queue.qMap, queue.q)
			}

			i, err := task.getter()
			r := Result{i, err}

			for _, v := range task.Receivers {
				v <- r
			}

			if err == nil {
				updateCache(key, i)
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


func resetCache() {
	defer mtxCache.Unlock()
	mtxCache.Lock()
	cacheMap = make(map[string]*Cache)
}

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	resetCache()

	queue = Queue{
		qMap: make(map[string]*SomeStruct),
		q:    []string{},
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
