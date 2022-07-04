package main

import (
	"fmt"
	"github.com/ambeloe/gospot"
)

func main() {
	sess, _ := gospot.Login("conf.json", false)

	ts, _ := sess.GetLikedSongs()

	for i, t := range ts {
		if t.Id == "" {
			fmt.Println("fug", i)
		}
	}
}
