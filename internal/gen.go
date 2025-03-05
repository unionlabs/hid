package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

func main() {
	hidapiIncludes := make(map[string]string)
	loadIncludes(hidapiIncludes, "./hidapi/hidapi")

	// 	hidapi/mac/hid.c -> hidapi_mac.h
	inlineTo(
		"hidapi/mac/hid.c",
		hidapiIncludes,
		"hidapi_mac.h",
	)

	// hidapi/windows/hid.c -> hidapi_windows.h
	inlineTo(
		"hidapi/windows/hid.c",
		hidapiIncludes,
		"hidapi_windows.h",
	)

	fmt.Println("hidapi_linux.h:")
	outF, err := os.Create("hidapi_linux.h")
	if err != nil {
		panic(err)
	}
	defer outF.Close()
	out := bufio.NewWriter(outF)
	defer out.Flush()

	/*
		// -I./hidapi/hidapi -I./libusb/libusb
		#include <poll.h>
		#include "os/threads_posix.c"
		#include "os/poll_posix.c"

		#include "os/linux_usbfs.c"
		#include "os/linux_netlink.c"

		#include "core.c"
		#include "descriptor.c"
		#include "hotplug.c"
		#include "io.c"
		#include "strerror.c"
		#include "sync.c"

		#include "hidapi/libusb/hid.c"
	*/
	linuxIncludes := make(map[string]string)
	loadIncludes(linuxIncludes, "./hidapi/hidapi")
	loadIncludes(linuxIncludes, "./libusb/libusb")
	seen := make(map[string]bool)

	out.WriteString("#include <poll.h>\n")
	for _, f := range []string{
		"os/threads_posix.c",
		"os/poll_posix.c",

		"os/linux_usbfs.c",
		"os/linux_netlink.c",

		"core.c",
		"descriptor.c",
		"hotplug.c",
		"io.c",
		"strerror.c",
		"sync.c",
	} {
		inline(
			"libusb/libusb/"+f,
			linuxIncludes,
			out,
			seen,
		)
	}
	inline(
		"hidapi/libusb/hid.c",
		linuxIncludes,
		out,
		seen,
	)
}

func loadIncludes(m map[string]string, dir string) {
	d, err := os.Open(dir)
	if err != nil {
		panic(err)
	}
	names, err := d.Readdirnames(0)
	if err != nil {
		panic(err)
	}
	for _, name := range names {
		m[name] = filepath.Join(dir, name)
	}
}

func inlineTo(src string, includes map[string]string, out string) error {
	fmt.Printf("%s:\n", src)
	outF, err := os.Create(out)
	if err != nil {
		return err
	}
	defer outF.Close()
	return inline(src, includes, bufio.NewWriter(outF), map[string]bool{})
}

var reInclude = regexp.MustCompile(`^ *#include *([<"])([^">]+)[">]`)

func inline(src string, includes map[string]string, w *bufio.Writer, seen map[string]bool) error {
	if seen[src] {
		fmt.Println("  SKIP ", src)
		fmt.Fprintf(w, "// SKIP #include %q\n", src)
		return nil
	}
	fmt.Println(" ", src)

	seen[src] = true

	fmt.Fprintf(w, "#line 1 %q\n", src)

	dir := filepath.Dir(src)
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(srcF)
	linenum := 0
	for scanner.Scan() {
		line := scanner.Text()
		linenum++
		matches := reInclude.FindStringSubmatch(line)
		var target string
		if matches != nil {
			target = includes[matches[2]]
			if target == "" && matches[1][0] == '"' {
				target = filepath.Join(dir, matches[2])
			}
			if target != "" {
				if err = inline(target, includes, w, seen); err != nil {
					return fmt.Errorf("%s:%d: %w", src, linenum, err)
				}
				fmt.Fprintf(w, "#line %d %q\n", linenum+1, src)

				continue
			}
		}

		w.WriteString(line)
		w.WriteByte('\n')
	}
	w.Flush()
	return scanner.Err()
}
