package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	ip := flag.String("ip", "0.0.0.0:5200", "")
	flag.Parse()

	if len(os.Args) > 1 && strings.ToLower(os.Args[1]) == "daemon" {

		arr, _ := exec.Command("lsof", "-t", "Ginga.lock").Output()
		if len(arr) > 0 {
			fmt.Println(fmt.Sprintf("检测到已有Ginga程序运行, PID %s", string(arr)))
			_ = exec.Command("kill", string(arr)).Run()
		}

		// 碰到的第一个坑,父进程结束时,会向子进程发送HUP,TERM指令,导致子进程会跟随父进程一块结束.
		// SysProcAttr.Setpgid设置为true,使子进程的进程组ID与其父进程不同.(KILL强杀也可以)
		cmd := exec.Command(os.Args[0], os.Args[2:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		} else {
			log.Println(fmt.Sprintf("%s [PID] %d running...", os.Args[0], cmd.Process.Pid))
		}

		return
	}

	//LOCK_SH 共享锁,多个进程可以使用同一把锁,常用作读共享锁.
	//LOCK_EX 排他锁,同时只允许一个进程使用,常被用作写锁.
	//LOCK_UN 释放锁.
	//        如果文件被其他进程锁住,进程会被阻塞直到锁释放.
	//LOCK_NB 如果文件被其他进程锁住,会返回错误 EWOULDBLOCK
	lock, e := os.Create("Ginga.lock")
	if e != nil {
		log.Fatalln(e)
	}
	defer lock.Close()

	//gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.GET("/chunked", func(c *gin.Context) {

		c.Header("Content-Type", "text/html")
		c.Writer.WriteHeader(http.StatusOK)

		c.Writer.Write([]byte(`<html><body>`))
		c.Writer.Flush()

		for i := 0; i < 10; i++ {
			c.Writer.Write([]byte(fmt.Sprintf(`<h3>%d<h3>`, i)))
			c.Writer.Flush()
			time.Sleep(1 * time.Second)
		}

		c.Writer.Write([]byte(`</body></html>`))
		c.Writer.Flush()
	})

	srv := &http.Server{Addr: *ip, Handler: r}
	srv.RegisterOnShutdown(func() {
		log.Println(fmt.Sprintf("Server is shutting down"))
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Println(fmt.Sprintf("Server closed under request"))
			} else {
				log.Println(err)
			}
		}
	}()

	down := make(chan os.Signal, 1)
	signal.Notify(down, syscall.SIGINT, syscall.SIGTERM)
	<-down

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Println(err)
	}
	log.Println("Server has stopped gracefully.")
}
