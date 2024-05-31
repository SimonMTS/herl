package serve

import (
	"sync"
)

// TODO: origin without protocol causes crashes, and probably some more stuff
// TODO: try not to force 'http' in script
// TODO: variable naming

var (
	events    = make(chan struct{})
	listeners = 0
)

func Serve(quiet bool, wsBind string, proxyBind string, origin string) (err error) {
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		err = proxyServer(quiet, wsBind, proxyBind, origin)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		err = notificationServer(wsBind)
		wg.Done()
	}()

	wg.Wait()
	return err
}
