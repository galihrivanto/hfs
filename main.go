package main

import "log"

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
	}
}
