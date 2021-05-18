// +build linux freebsd dragonfly darwin

package shpnetpoll

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"shpnetpoll/errors"
	"shpnetpoll/internal/logging"
	"shpnetpoll/internal/netpoll"
)

type server struct {
	ln           *listener          // the listener for accepting new connections
	lb           loadBalancer       // event-loops for handling events
	wg           sync.WaitGroup     // event-loop close WaitGroup
	opts         *Options           // options with server
	once         sync.Once          // make sure only signalShutdown once
	cond         *sync.Cond         // shutdown signaler
	codec        ICodec             // codec for TCP stream
	logger       logging.Logger     // customized logger for logging info
	ticktock     chan time.Duration // ticker channel
	mainLoop     *eventloop         // main event-loop for accepting connections
	inShutdown   int32              // whether the server is in shutdown
	eventHandler EventHandler       // user eventHandler
}

var serverFarm sync.Map

func (svr *server) isInShutdown() bool {
	return atomic.LoadInt32(&svr.inShutdown) == 1
}

// waitForShutdown waits for a signal to shutdown.
func (svr *server) waitForShutdown() {
	svr.cond.L.Lock()
	svr.cond.Wait()
	svr.cond.L.Unlock()
}

// signalShutdown signals the server to shut down.
func (svr *server) signalShutdown() {
	svr.once.Do(func() {
		svr.cond.L.Lock()
		svr.cond.Signal()
		svr.cond.L.Unlock()
	})
}

func (svr *server) startEventLoops() {
	svr.lb.iterate(func(i int, el *eventloop) bool {
		svr.wg.Add(1)
		go func() {
			el.loopRun(svr.opts.LockOSThread)
			svr.wg.Done()
		}()
		return true
	})
}

func (svr *server) closeEventLoops() {
	svr.lb.iterate(func(i int, el *eventloop) bool {
		_ = el.poller.Close()
		return true
	})
}

func (svr *server) startSubReactors() {
	svr.lb.iterate(func(i int, el *eventloop) bool {
		svr.wg.Add(1)
		go func() {
			svr.activateSubReactor(el, svr.opts.LockOSThread)
			svr.wg.Done()
		}()
		return true
	})
}

func (svr *server) activateEventLoops(numEventLoop int) (err error) {
	// Create loops locally and bind the listeners.
	for i := 0; i < numEventLoop; i++ {
		l := svr.ln
		if i > 0 && svr.opts.ReusePort {
			if l, err = initListener(svr.ln.network, svr.ln.addr, svr.ln.reusePort); err != nil {
				return
			}
		}

		var p *netpoll.Poller
		if p, err = netpoll.OpenPoller(); err == nil {
			el := new(eventloop)
			el.ln = l
			el.svr = svr
			el.poller = p
			el.packet = make([]byte, svr.opts.ReadBufferCap)
			el.connections = make(map[int]*conn)
			el.eventHandler = svr.eventHandler
			el.calibrateCallback = svr.lb.calibrate
			_ = el.poller.AddRead(el.ln.fd)
			svr.lb.register(el)

			// Start the ticker.
			if el.idx == 0 && svr.opts.Ticker {
				go el.loopTicker()
			}
		} else {
			return
		}
	}

	// Start event-loops in background.
	svr.startEventLoops()

	return
}

func (svr *server) activateReactors(numEventLoop int) error {
	// 创建numEventLoop个eventLoop
	for i := 0; i < numEventLoop; i++ {
		// 为每一个eventLoop设置一个poller
		if p, err := netpoll.OpenPoller(); err == nil {
			el := new(eventloop)
			el.ln = svr.ln
			el.svr = svr
			el.poller = p
			el.packet = make([]byte, svr.opts.ReadBufferCap)
			el.connections = make(map[int]*conn)
			el.eventHandler = svr.eventHandler
			el.calibrateCallback = svr.lb.calibrate
			// 将eventLoop注册到 负载均衡上
			svr.lb.register(el)

			// Start the ticker.
			if el.idx == 0 && svr.opts.Ticker {
				go el.loopTicker()
			}
		} else {
			return err
		}
	}

	// 开启从Reactor
	// Start sub reactors in background.
	svr.startSubReactors()

	// 创建eventLoop
	if p, err := netpoll.OpenPoller(); err == nil {
		el := new(eventloop)
		el.ln = svr.ln
		el.idx = -1
		el.svr = svr
		// 在这里将epoll fd 赋值给主Reactor
		el.poller = p
		// 在这里将listener 监听的fd添加到epoll的读事件
		_ = el.poller.AddRead(el.ln.fd)
		svr.mainLoop = el

		// Start main reactor in background.
		svr.wg.Add(1)
		go func() {
			// 开启主Reactor， 主Reactor什么时候进程运行呢？
			svr.activateMainReactor(svr.opts.LockOSThread)
			svr.wg.Done()
		}()
	} else {
		return err
	}

	return nil
}

func (svr *server) start(numEventLoop int) error {
	if svr.opts.ReusePort || svr.ln.network == "udp" {
		return svr.activateEventLoops(numEventLoop)
	}

	// 启动reactor模式
	return svr.activateReactors(numEventLoop)
}

func (svr *server) stop(s Server) {
	// Wait on a signal for shutdown
	svr.waitForShutdown()

	svr.eventHandler.OnShutdown(s)

	// Notify all loops to close by closing all listeners
	svr.lb.iterate(func(i int, el *eventloop) bool {
		sniffErrorAndLog(el.poller.Trigger(func() error {
			return errors.ErrServerShutdown
		}))
		return true
	})

	if svr.mainLoop != nil {
		svr.ln.close()
		sniffErrorAndLog(svr.mainLoop.poller.Trigger(func() error {
			return errors.ErrServerShutdown
		}))
	}

	// Wait on all loops to complete reading events
	svr.wg.Wait()

	svr.closeEventLoops()

	if svr.mainLoop != nil {
		sniffErrorAndLog(svr.mainLoop.poller.Close())
	}

	// Stop the ticker.
	if svr.opts.Ticker {
		close(svr.ticktock)
	}

	atomic.StoreInt32(&svr.inShutdown, 1)
}

func serve(eventHandler EventHandler, listener *listener, options *Options, protoAddr string) error {
	// Figure out the proper number of event-loops/goroutines to run.
	numEventLoop := 1
	if options.Multicore {
		numEventLoop = runtime.NumCPU()
	}
	if options.NumEventLoop > 0 {
		numEventLoop = options.NumEventLoop
	}

	svr := new(server)
	svr.opts = options
	svr.eventHandler = eventHandler
	// 设置监听器
	svr.ln = listener

	// 负载均衡
	switch options.LB {
	case RoundRobin:
		svr.lb = new(roundRobinLoadBalancer)
	case LeastConnections:
		svr.lb = new(leastConnectionsLoadBalancer)
	case SourceAddrHash:
		svr.lb = new(sourceAddrHashLoadBalancer)
	}

	svr.cond = sync.NewCond(&sync.Mutex{})
	svr.ticktock = make(chan time.Duration, channelBuffer)
	svr.logger = logging.DefaultLogger
	svr.codec = func() ICodec {
		if options.Codec == nil {
			return new(BuiltInFrameCodec)
		}
		return options.Codec
	}()

	server := Server{
		svr:          svr,
		Multicore:    options.Multicore,
		Addr:         listener.lnaddr,
		NumEventLoop: numEventLoop,
		ReusePort:    options.ReusePort,
		TCPKeepAlive: options.TCPKeepAlive,
	}
	switch svr.eventHandler.OnInitComplete(server) {
	case None:
	case Shutdown:
		return nil
	}
	// 开启服务
	if err := svr.start(numEventLoop); err != nil {
		svr.closeEventLoops()
		svr.logger.Errorf("gnet server is stopping with error: %v", err)
		return err
	}
	defer svr.stop(server)

	serverFarm.Store(protoAddr, svr)

	return nil
}
