package main

import (
	"instagramscrapper/core"

	"github.com/gin-gonic/gin"
)

func main() {

	running := false
	var s *core.InstagramScrapper
	r := gin.Default()

	r.POST("/config", func(c *gin.Context) {

		if !running || s == nil {
			s = core.NewInstagramScrapper()
		}

		var startupData core.StartupData

		c.BindJSON(&startupData)

		startupData.Cookie = c.GetHeader("Cookie")

		s.SetStartUpData(startupData)

		c.Status(200)

		if !running {
			running = true
			go s.Run()
		}

		return
	})

	r.Run(":5555")

}
