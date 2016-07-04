package etpi

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type Addr struct {
	NetworkString string
	AddrString    string
}

func (a Addr) Network() string {
	return a.NetworkString
}

func (a Addr) String() string {
	return a.AddrString
}

type EndPoint struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func (e *EndPoint) Read(data []byte) (n int, err error)  { return e.r.Read(data) }
func (e *EndPoint) Write(data []byte) (n int, err error) { return e.w.Write(data) }
func (e *EndPoint) Close() error {
	var err error
	err = e.r.Close()
	err = e.w.Close()
	return err
}

func (e *EndPoint) LocalAddr() net.Addr {
	return Addr{
		NetworkString: "tcp",
		AddrString:    "127.0.0.1",
	}
}

func (e *EndPoint) RemoteAddr() net.Addr {
	return Addr{
		NetworkString: "tcp",
		AddrString:    "127.0.0.1",
	}
}

func (e *EndPoint) SetDeadline(t time.Time) error      { return nil }
func (e *EndPoint) SetReadDeadline(t time.Time) error  { return nil }
func (e *EndPoint) SetWriteDeadline(t time.Time) error { return nil }

type MockConn struct {
	Client *EndPoint
	Server *EndPoint
}

func NewMockConn() *MockConn {
	serverRead, clientWrite := io.Pipe()
	clientRead, serverWrite := io.Pipe()
	return &MockConn{
		Client: &EndPoint{r: clientRead, w: clientWrite},
		Server: &EndPoint{r: serverRead, w: serverWrite},
	}
}

func (c *MockConn) Close() error {
	if err := c.Server.Close(); err != nil {
		return err
	}
	if err := c.Client.Close(); err != nil {
		return err
	}
	return nil
}

func TestClientLogin(t *testing.T) {
	conn := NewMockConn()
	c := NewClient().(*client)
	c.conn = conn.Client
	c.pwd = "user"
	var w sync.WaitGroup
	w.Add(1)
	go func() {
		r := bufio.NewReader(conn.Server)
		line, err := r.ReadBytes('\n')
		if err != nil || bytes.Compare(line, []byte("005user54\r\n")) != 0 {
			t.Error(err, line)
		}
		w.Done()
	}()
	err := c.login()
	w.Wait()
	if err != nil {
		t.Error(err)
	}
}

func TestListen(t *testing.T) {
	conn := NewMockConn()
	c := NewClient().(*client)
	c.conn = conn.Client
	go c.listen()
	c.Disconnect()
}

func TestClientLoginRequest(t *testing.T) {
	conn := NewMockConn()
	c := NewClient().(*client)
	c.conn = conn.Client
	c.pwd = "user"
	var w sync.WaitGroup
	w.Add(1)
	go c.listen()
	r := bufio.NewReader(conn.Server)
	cmd := Command{Code: "505", Data: "3"}
	cmd.WriteTo(conn.Server)
	line, err := r.ReadString('\n')
	if err != nil || line != "005user54\r\n" {
		t.Error(err, line)
	}
}
