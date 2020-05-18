package modbus

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

func TestServer_Serve(t *testing.T) {

	s := NewServer()

	s.Handler = func(c *Conn, out []byte) {
		// handle response
	}
	s.AfterConnClose = func(sn string) {
		// do something
	}

	s.Debug(true)

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
}
