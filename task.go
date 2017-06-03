package testTask

import (
	"sync"
	"errors"
	"time"
	"log"
	"fmt"
)

var (
	cacheMap map[string]*Cache
	mtxCache sync.Mutex

	//todo: ttl инфы в кэше
	validTimeSeconds int64 = 3

	//todo: обновление кэша нет
	getterMap map[string]*Getter
	mtxGMap   sync.Mutex
)

type Cache struct {
	data         *interface{}
	creationTime int64
}

// Очередь сделана в виде слайса строк-ключей для удобной итерации
// и мапа для хранения всяких нужных вещей

type Getter struct {
	key      string
	getter   func() (interface{}, error)
	chResult chan Result

	receiversCount int
	*sync.Mutex
}

func (s *Getter) addReceiver() {
	defer s.Unlock()
	s.Lock()
	s.receiversCount++
}

func (s *Getter) removeReceiver() {
	defer s.Unlock()
	s.Lock()
	s.receiversCount--
}

func (s *Getter) haveReceivers() bool {
	defer s.Unlock()
	s.Lock()
	return s.receiversCount != 0
}

func (s *Getter) get() {
	defer close(s.chResult)
	i, err := s.getter()

	r := Result{i, err}
	if err == nil {
		updateCache(s.key, i)
	}

	// удаляем из мапы, чтобы во время рассылки не набрались новые получатели
	// (рассылка может стать неактуальной)
	func() {
		defer mtxGMap.Unlock()
		mtxGMap.Lock()
		delete(getterMap, s.key)
	}()

	for s.haveReceivers() {
		s.chResult <- r
		s.removeReceiver()
	}
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

func resetCache() {
	defer mtxCache.Unlock()
	mtxCache.Lock()
	cacheMap = make(map[string]*Cache)
}

func updateCache(key string, i interface{}) {
	defer mtxCache.Unlock()
	mtxCache.Lock()

	cacheMap[key] = &Cache{
		&i,
		time.Now().Unix(),
	}
}

func getResult(key string, getter func() (interface{}, error)) chan Result {
	defer mtxGMap.Unlock()
	mtxGMap.Lock()

	v, ok := getterMap[key]
	if ok {
		v.addReceiver()
		return v.chResult
	}

	g := &Getter{
		key,
		getter,
		make(chan Result),
		0,
		&sync.Mutex{},
	}
	getterMap[key] = g
	g.addReceiver()

	go g.get()

	return g.chResult
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

	result := <-getResult(key, getter)
	i = result.i
	err = result.err
	return
}

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	resetCache()
	getterMap = make(map[string]*Getter)

	go func() {
		for {
			select {
			case <-time.After(100 * time.Millisecond):
				eraseCache()
			}
		}
	}()
}
