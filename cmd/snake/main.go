package main

import (
	"fmt"

	"github.com/oxaxxaxaxaxaxaxxaaaxax/snake/internal/application"
)

func main() {
	fmt.Println("Start! Choose mode:")
	fmt.Println("1. New game")
	fmt.Println("2. Join game")
	var mode int
	fmt.Scanf("%d", &mode)
	switch mode {
	case 1:
		fmt.Println("New game started")
		master := application.NewMaster(&application.Engine{})
		master.Engine.StartGame(master)
	case 2:
		fmt.Println("Choose role")
		fmt.Println("1. normal")
		fmt.Println("2. viewer")
		var role int
		fmt.Scanf("%d", &role)
		switch role {
		case 1:
			fmt.Println("Join Normal game started")
			normal := application.NewNormal(&application.Engine{})
			normal.Engine.StartGame(normal)
		case 2:
			fmt.Println("Join Viewer game started")
			viewer := application.NewViewer(&application.Engine{})
			viewer.Engine.StartGame(viewer)
		}
	}
}
