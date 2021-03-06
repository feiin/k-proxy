package main

import (
	"net/url"
	"net/http/httputil"
	"net/http"
	"fmt"
	"flag"
	"time"
	"net"
)

//重新定义net.Listener
type counterListener struct {
	net.Listener
}

//重写net.Listener.Accept(),对接收到的连接注入请求计数器
func (c *counterListener) Accept() (net.Conn, error) {
	conn, err := c.Listener.Accept()
	// fmt.Printf("new connection %+v", conn)
	if err != nil {
		return nil, err
	}
	return &counterConn{Conn: conn}, nil
}

//定义计数器counter和计数方法Increment()
type counter int

func (c *counter) Increment() int {
	*c++
	return int(*c)
}

func (c *counter) GetRequestsCounter() int {
	return int(*c)
}

//重新定义net.Conn,注入计数器ct
type counterConn struct {
	net.Conn
	ct counter
}

//重写net.Conn.LocalAddr()，返回本地网络地址的同时返回该连接累计处理过的请求数
func (c *counterConn) LocalAddr() net.Addr {
	return &counterAddr{c.Conn.LocalAddr(), &c.ct}
}

//定义TCP连接计数器,指向连接累计请求的计数器
type counterAddr struct {
	net.Addr
	*counter
}

//ReverseProxy 代理请求
func ReverseProxy(targets []*url.URL) *httputil.ReverseProxy{
 	director:= func(req *http.Request) {
 		target:=targets[0]
 		req.URL.Scheme = target.Scheme
 		req.URL.Host = target.Host

		// fmt.Printf("req
		localAddr := req.Context().Value(http.LocalAddrContextKey)
		if ct, ok := localAddr.(interface{ Increment() int }); ok {
			ct.Increment()
			//just incr
		}
		
		
	}

	modifyResponse := func(r *http.Response)  error{
		localAddr := r.Request.Context().Value(http.LocalAddrContextKey)
		if ct, ok := localAddr.(interface{ GetRequestsCounter() int }); ok {
			currRequests := ct.GetRequestsCounter()
			// fmt.Printf("response current number %d requests  %d,\r\n",currRequests,maxRequestsPerCon)
			if currRequests >= maxRequestsPerCon {
				r.Header.Set("Connection", "close")
			}
		}
		return nil
	}

	roundTripper := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   200,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
 	return &httputil.ReverseProxy{
		 Director:director,
		 Transport:roundTripper,
		 ModifyResponse:modifyResponse,
	}
}

var maxRequestsPerCon = 1000
func main() {

	targetPort := flag.Int("target_service_port", 3000, "转发目标端口")
	targetHost := flag.String("target_service_host", "127.0.0.1", "转发目标HOST")
	listenPort := flag.Int("listen_port", 1616, "本机监听端口")
	requestsPerCon := flag.Int("requests_per_conn", 1000, "每个conn最大http请求数")

	flag.Parse()

	

	maxRequestsPerCon=*requestsPerCon
	targetReverseHost := fmt.Sprintf("%s:%d",*targetHost,*targetPort)
	proxy:=ReverseProxy([]*url.URL{
		{
			Scheme:"http",
			Host: targetReverseHost,
		},
	}) 

	l, err := net.Listen("tcp", fmt.Sprintf(":%d",*listenPort))
	if err != nil {
		panic(err)
	}
	err = http.Serve(&counterListener{l}, proxy)
	if err != nil {
		panic(err)
	}
}