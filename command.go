package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	rootCmd *cobra.Command
	option  *Option
	appName string
)

func init() {

	appName = filepath.Base(os.Args[0])

	viper.SetConfigType("yaml")
	viper.SetConfigName(appName)
	viper.AddConfigPath(fmt.Sprintf("/etc/%s/", appName))
	viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", appName))
	viper.AddConfigPath(".")

	option = &Option{}

	cobra.OnInitialize(func() {

		if err := viper.ReadInConfig(); err != nil {
			log.Println("config not file found, use default")
		}

		if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
			log.Println(err)
		}

		if err := viper.Unmarshal(option); err != nil {
			log.Println(err)
		}

		if !option.Verbose {
			log.SetOutput(ioutil.Discard)
		}

		if option.Root == "" {
			option.Root = "."
		}

		if option.ServeAddress == "" {
			option.ServeAddress = ":3030"
		}

	})

	rootCmd = &cobra.Command{
		Use:   appName,
		Short: "Http file sharing",
		Run: func(cmd *cobra.Command, args []string) {
			runServer()
		},
	}
}
