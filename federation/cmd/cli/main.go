/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"

	"github.com/vmware-tanzu/antrea/federation/cli"
)

var (
	nonInteractive = flag.Bool("non-interactive", false, "")
)

func main() {
	flag.Parse()
	c, err := cli.NewCompleter()
	if err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
	if *nonInteractive {
		c.(cli.Executor).Executor(strings.Join(os.Args, ""))
		return
	}

	fmt.Println("Please use `exit` or `Ctrl-D` to exit this program.")
	defer fmt.Println("Bye!")
	p := prompt.New(
		c.(cli.Executor).Executor,
		c.Complete,
		prompt.OptionTitle("antrea+cli: interactive Antrea+ CLI"),
		prompt.OptionPrefix("Antrea>> "),
		prompt.OptionInputTextColor(prompt.Yellow),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator),
		prompt.OptionAddKeyBind(
			prompt.KeyBind{
				Key: prompt.ControlF,
				Fn:  prompt.GoRightWord,
			},
			prompt.KeyBind{
				Key: prompt.ControlB,
				Fn:  prompt.GoLeftWord,
			},
		),
	)
	p.Run()
}
