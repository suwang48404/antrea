/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package cli

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/vmware-tanzu/antrea/federation/cli/kube-prompt"
)

func (c *rootCompleter) Executor(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	} else if s == "quit" || s == "exit" {
		fmt.Println("Bye!")
		os.Exit(0)
	}
	fields := strings.Fields(s)
	switch fields[0] {
	case argCluster:
		if len(fields) < 3 {
			fmt.Printf("Unsupported command:  %s\n", s)
			return
		}
		s := "--context " + c.clusterContextMap[fields[1]] + " " + strings.Join(fields[2:], " ")
		kube.Executor(s)
		cmd := exec.Command("/bin/sh", "-c", s)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	case argDiscoverCluster:
		c.MergeClusters()
	case argListClusters:
		out, err := c.kubectl.Cmd("config get-contexts")
		if err != nil {
			fmt.Printf("Cannot list clusters: %v", err)
			return
		}
		lines := strings.Split(out, "\n")
		fmt.Println(lines[0])
		ctxs := make([]string, 0, len(c.clusterContextMap))
		for _, ctx := range c.clusterContextMap {
			ctxs = append(ctxs, ctx)
		}
		r := regexp.MustCompile(strings.Join(ctxs, "|"))
		for i := 1; i < len(lines)-1; i++ {
			l := lines[i]
			if !r.MatchString(l) {
				continue
			}
			println(l)
		}
		fmt.Println(lines[len(lines)-1])
	case argFederation:
		if len(fields) < 3 {
			fmt.Printf("Unsupported federation command:  %s\n", s)
		}
		fed, ok := c.federations[fields[1]]
		if !ok {
			fed = c.tmpFederation
		}
		if fed == nil {
			// TODO, It is possible in non-interactive mode, disallow for now.
			fmt.Printf("Unknown federation:  %s\n", s)
			return
		}
		fed.Executor(fields[2], fields[3:]...)
		if fields[2] == fedActionAdd || fields[2] == fedActionRemove {
			c.initializeFederations()
		}
	case argListFederations:
		for n, fed := range c.federations {
			fmt.Println(n)
			for cl := range fed.getClusters(true, false) {
				fmt.Println("\t" + cl)
			}
		}
	default:
		fmt.Printf("Unsupported command:  %s\n", s)
	}
	c.resetResource()
}

func (c *rootCompleter) MergeClusters() {
	dir := ""
	if f := flag.Lookup(clientcmd.RecommendedConfigPathFlag); f != nil {
		dir = f.Value.(flag.Getter).Get().(string)
	}
	if len(dir) == 0 {
		dir = os.Getenv(clientcmd.RecommendedConfigPathEnvVar)
	}
	if len(dir) > 0 {
		dir = path.Dir(strings.Split(dir, ":")[0])
	} else {
		dir = clientcmd.RecommendedConfigDir
	}
	fmt.Printf("Retrieve kubeconfig files from %s\n", dir)
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Printf("Cannot read from %s: %v", dir, err)
		return
	}
	kconfigFiles := ""
	for _, i := range infos {
		if i.IsDir() {
			continue
		}
		kconfigFiles += path.Join(dir, i.Name()) + ":"
	}
	kconfigFiles = kconfigFiles[:len(kconfigFiles)-1]
	config := path.Join(dir, clientcmd.RecommendedFileName)
	newConfig := "/tmp/kubeconfig" + "." + time.Now().Format(time.RFC3339)
	cmd := exec.Command("/bin/bash", "-c",
		fmt.Sprintf("kubectl config view --merge --flatten > %s", newConfig))
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", clientcmd.RecommendedConfigPathEnvVar, kconfigFiles))
	if err := cmd.Run(); err != nil {
		fmt.Printf("Merge kubeconfig files in %s failed: %v\n", dir, err)
		return
	}
	backup := newConfig + ".bak"
	fmt.Printf("Move old kubeconfig to %s\n", backup)
	_ = os.Remove(backup)
	_ = os.Rename(config, backup)
	_ = os.Rename(newConfig, config)
	_ = c.initialize()
}
