# mbserver

fork from tbrandon/mbserver

modbus协议的TCP连接 go实现

依赖：zap日志

## Usage
```go
    // 设置zap log
	Logger=zap.NewExample()

    s := NewServer()
	s.Handler = func(c *Conn, out []byte) {
		// handle response
	}
	s.AfterConnClose = func(sn string) {
		// do something
	}
	s.OnStart = func() {
		// do something
	}

	go func() {
		err := s.StartServer(":6500")
		if err != nil {
			log.Print(err.Error())
		}
	}()

	// gracefully shutdown
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 10 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit
	s.Shutdown()
``` 