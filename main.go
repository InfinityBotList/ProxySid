package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

var localAddr net.Addr

func main() {
	// Get reserved IP from DO
	res, err := http.Get("http://169.254.169.254/metadata/v1/interfaces/public/0/anchor_ipv4/address")
	if err != nil {
		panic(err)
	}

	text, err := io.ReadAll(res.Body)

	if err != nil {
		panic(err)
	}

	fmt.Println("Got IP (from DO) of:", string(text))

	localAddr, err = net.ResolveTCPAddr("tcp", string(text)+":0")

	if err != nil {
		panic(err)
	}

	client := newClient()

	app := fiber.New()

	app.Get("/proxy", func(c *fiber.Ctx) error {
		return c.SendString("http://127.0.0.1:3000")
	})

	app.All("/*", func(c *fiber.Ctx) error {
		req, err := http.NewRequest(c.Method(), "https://discord.com/"+c.Params("*"), bytes.NewReader(c.Body()))

		if err != nil {
			c.Append("X-Proxy-Error", err.Error())
			return c.SendStatus(502)
		}

		headers := c.GetReqHeaders()

		if v, ok := headers["X-Audit-Log-Reason"]; ok {
			req.Header.Set("X-Audit-Log-Reason", v)
		}

		if v, ok := headers["Authorization"]; ok {
			req.Header.Set("Authorization", v)
		}

		if v, ok := headers["Content-Type"]; ok {
			req.Header.Set("Content-Type", v)
		}

		if v, ok := headers["User-Agent"]; ok {
			req.Header.Set("User-Agent", v+" (proxysid")
		}

		resp, err := client.Do(req)

		if err != nil {
			c.Append("X-Proxy-Error", err.Error())
			return c.SendStatus(502)
		}

		c.Status(resp.StatusCode)

		for k, v := range resp.Header {
			c.Append(k, v...)
		}

		fmt.Println(c.Method(), c.Params("*"), "returns", resp.StatusCode)

		c.SendStream(resp.Body)

		return nil
	})

	err = app.Listen(":65535")

	if err != nil {
		panic(err)
	}
}

func newClient() http.Client {
	// Create a transport like http.DefaultTransport, but with a specified localAddr
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			LocalAddr: localAddr,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
	}
}
