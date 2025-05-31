package elk

import (
	"TaipeiCityDashboardBE/global"
	"TaipeiCityDashboardBE/logs"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

var MessageWorker *WorkerPool

func InitWorkerPool() {
	// Step 1: create ELK TCP connection pool
	connPool := NewConnectionPool(global.ELK.LogstashURL, global.ELK.ConnectionMaxIdle, 5*time.Second, 30*time.Second)

	// Step 2: create worker pool
	workerPool := NewWorkerPool(global.ELK.BufferSize, global.ELK.WorkerCount, connPool)
	MessageWorker = workerPool

	// Step 3: log config
	configJsonBytes, err := json.Marshal(global.ELK)
	if err != nil {
		logs.Error("error marshaling struct: %v", err)
	}
	logs.Info(string(configJsonBytes))
}

type ElkMessage struct {
	Time            time.Time     `json:"@timestamp"`
	LogLevel        logs.LogLevel `json:"log_level"`
	ApiRoute        string        `json:"api_route"`
	RequestUrl      string        `json:"request_url"`
	Message         string        `json:"message"`
	ApplicationName string        `json:"application_name"`
	SourceIP        string        `json:"source_ip"`
	Latency         int64         `json:"latency"`
	HttpStatus      int           `json:"http_status"`
	HttpMethod      string        `json:"http_method"`
	RequestBody     string        `json:"request_body"`
	ResponseBody    string        `json:"response_body"`
}

// 所有 payload 都要是 struct
type ELKPayload interface{}

type MessageRingBuffer struct {
	buffer   []ELKPayload
	size     int
	head     int
	tail     int
	count    int
	mutex    sync.Mutex
	notEmpty *sync.Cond
	notFull  *sync.Cond
}

func NewRingBuffer(size int) *MessageRingBuffer {
	rb := &MessageRingBuffer{
		buffer: make([]ELKPayload, size),
		size:   size,
	}
	rb.notEmpty = sync.NewCond(&rb.mutex)
	rb.notFull = sync.NewCond(&rb.mutex)
	return rb
}

func (rb *MessageRingBuffer) Pub(msg ELKPayload) {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()
	for rb.count >= rb.size {
		rb.notFull.Wait()
	}
	rb.buffer[rb.tail] = msg
	rb.tail = (rb.tail + 1) % rb.size
	rb.count++
	rb.notEmpty.Signal()
}

func (rb *MessageRingBuffer) Sub() ELKPayload {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()
	for rb.count == 0 {
		rb.notEmpty.Wait()
	}
	msg := rb.buffer[rb.head]
	rb.head = (rb.head + 1) % rb.size
	rb.count--
	rb.notFull.Signal()
	return msg
}

type ConnectionPool struct {
	url         string
	maxIdle     int
	mu          sync.Mutex
	idleConns   chan net.Conn
	dialTimeout time.Duration
	ioTimeout   time.Duration
	closed      chan struct{}
}

func NewConnectionPool(url string, maxIdle int, dialTimeout time.Duration, ioTimeout time.Duration) *ConnectionPool {
	pool := &ConnectionPool{
		url:         url,
		maxIdle:     maxIdle,
		idleConns:   make(chan net.Conn, maxIdle),
		dialTimeout: dialTimeout,
		ioTimeout:   ioTimeout,
		closed:      make(chan struct{}),
	}
	for i := 0; i < maxIdle; i++ {
		if conn, err := pool.dial(); err == nil {
			pool.idleConns <- conn
		}
	}
	go pool.healthChecker()
	return pool
}

func (p *ConnectionPool) dial() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", p.url, p.dialTimeout)
	if err == nil {
		logs.Info("new connection success, url: %s", p.url)
	} else {
		logs.Warn("new connection fail, url: %s, error: %v", p.url, err)
	}
	return conn, err
}

func (p *ConnectionPool) Get() (net.Conn, error) {
	select {
	case conn := <-p.idleConns:
		return conn, nil
	default:
		return p.dial()
	}
}

func (p *ConnectionPool) Put(conn net.Conn) {
	if conn == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case p.idleConns <- conn:
	default:
		conn.Close()
	}
}

func (p *ConnectionPool) healthChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			size := len(p.idleConns)
			p.mu.Unlock()
			for i := size; i < p.maxIdle; i++ {
				if conn, err := p.dial(); err == nil {
					p.idleConns <- conn
				}
			}
		case <-p.closed:
			p.closeAllConnection()
			return
		}
	}
}

func (p *ConnectionPool) close() {
	close(p.closed)
}

type WorkerPool struct {
	queue      *MessageRingBuffer
	wg         sync.WaitGroup
	poolConn   *ConnectionPool
	workerSize int
	closed     chan struct{}
}

func NewWorkerPool(queueSize, workerSize int, poolConn *ConnectionPool) *WorkerPool {
	wp := &WorkerPool{
		queue:      NewRingBuffer(queueSize),
		poolConn:   poolConn,
		workerSize: workerSize,
		closed:     make(chan struct{}),
	}
	wp.start()
	return wp
}

func (wp *WorkerPool) start() {
	for i := 0; i < wp.workerSize; i++ {
		wp.wg.Add(1)
		go func(id int) {
			defer wp.wg.Done()
			for {
				select {
				case <-wp.closed:
					return
				default:
					evt := wp.queue.Sub()
					if err := wp.handle(evt); err != nil {
						logs.Error("worker handle error: s\n", id, err)
					}
				}
			}
		}(i)
	}
}

func (wp *WorkerPool) Submit(evt ELKPayload) error {
	select {
	case <-wp.closed:
		return errors.New("worker pool is closed")
	default:
		wp.queue.Pub(evt)
		logs.Debug("submit event: ", evt)
		return nil
	}
}

func (wp *WorkerPool) handle(evt ELKPayload) error {
	conn, err := wp.poolConn.Get()
	if err != nil {
		return err
	}
	defer wp.poolConn.Put(conn)

	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(wp.poolConn.ioTimeout))
	_, err = io.WriteString(conn, string(data)+"\n")
	if err != nil {
		logs.Warn("ELK write msg error: \n, elk connection close", err)
		conn.Close()
		newConn, derr := wp.poolConn.dial()
		if derr != nil {
			return derr
		}
		wp.poolConn.Put(newConn)
		return err
	}
	return nil
}

func (p *ConnectionPool) closeAllConnection() {
	logs.Info("close all connection")
	for {
		select {
		case conn := <-p.idleConns:
			conn.Close()
		default:
			return
		}
	}
}

func (wp *WorkerPool) close() {
	close(wp.closed)
}

func (wp *WorkerPool) Shutdown() {
	wp.close()
	wp.poolConn.close()
	logs.Info("worker pool & connection pool shutdown")
	wp.wg.Wait()
}
