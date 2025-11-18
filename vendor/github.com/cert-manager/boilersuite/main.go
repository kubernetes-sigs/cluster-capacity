/*
Copyright 2023 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"

	"github.com/cert-manager/boilersuite/internal/boilersuite"
	"github.com/cert-manager/boilersuite/internal/version"
)

const (
	defaultAuthor = "cert-manager"
)

var (
	alwaysSkippedDirs = []string{".git", "_bin", "bin", "node_modules", "vendor", "third_party", "staging"}
)

//go:embed boilerplate-templates/*.boilertmpl
var boilerplateTemplateDir embed.FS

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	verboseLogger := log.New(io.Discard, "", 0)

	skipFlag := flag.String("skip", "", "Space-separated list of prefixes for paths which shouldn't be checked. Spaces in prefixes not supported.")
	authorFlag := flag.String("author", defaultAuthor, fmt.Sprintf("The expected author for files, which will be substituted for the %q marker in templates", boilersuite.AuthorMarkerRegex))
	verboseFlag := flag.Bool("verbose", false, "If set, prints verbose output")
	cpuProfile := flag.String("cpuprofile", "", "If set, writes CPU profiling information to the given filename")
	printVersion := flag.Bool("version", false, "If set, prints the version and exits")

	flag.Parse()

	if *printVersion {
		logger.Printf("version: %s", version.AppVersion)
		logger.Printf(" commit: %s", version.AppGitCommit)
		os.Exit(0)
	}

	if flag.NArg() != 1 {
		logger.Fatalf("usage: %s [--version] [--skip \"paths to skip\"] [--author \"example\"] [--verbose] <path-to-dir>", os.Args[0])
	}

	var skippedDirs []string

	if skipFlag != nil && len(*skipFlag) > 0 {
		skippedDirs = strings.Fields(*skipFlag)
	}

	if *verboseFlag {
		verboseLogger = log.New(os.Stdout, "[VERBOSE] ", log.LstdFlags)
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			logger.Fatal(err)
		}

		err = pprof.StartCPUProfile(f)
		if err != nil {
			logger.Fatal(err)
		}

		defer pprof.StopCPUProfile()
	}

	templates, err := boilersuite.LoadTemplates(boilerplateTemplateDir, *authorFlag)
	if err != nil {
		logger.Fatalf("failed to load templates: %s", err.Error())
	}

	targetBase := flag.Arg(0)

	dir, err := isDir(targetBase)
	if err != nil {
		// couldn't check if the base was a dir or not
		logger.Fatalf("target invalid: %s", err)
	}

	var targets []target

	if dir {
		targets, err = getTargets(targetBase, templates, skippedDirs, verboseLogger)
		if err != nil {
			logger.Fatalf("failed to list targets in dir %q: %s", targetBase, err.Error())
		}
	} else {
		contents, err := os.ReadFile(targetBase)
		if err != nil {
			logger.Fatalf("failed to read %q: %s", targetBase, err.Error())
		}

		targets = []target{target{
			path:     targetBase,
			contents: string(contents),
		}}
	}

	if len(targets) == 0 {
		return
	}

	validationErrors := make([]error, 0)

	for _, t := range targets {
		tmpl, ok := templates.TemplateFor(t.path)
		if !ok {
			panic("failed to get a template for a target which was already processed")
		}

		err := tmpl.Validate(t.contents)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("invalid boilerplate in %q: %w", t.path, err))
			continue
		}

		verboseLogger.Printf("validated %q successfully", t.path)
	}

	if len(validationErrors) == 0 {
		verboseLogger.Printf("all files validated successfully")
		return
	}

	for _, validationErr := range validationErrors {
		logger.Println(validationErr)
	}

	logger.Fatalln("at least one file had errors")
}

type target struct {
	path     string
	contents string
}

func isDir(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return stat.IsDir(), nil
}

func getTargets(targetBase string, templates boilersuite.TemplateMap, skippedPrefixes []string, verboseLogger *log.Logger) ([]target, error) {
	var targets []target

	skipMap := make(map[string]struct{})

	for _, skip := range append(skippedPrefixes, alwaysSkippedDirs...) {
		skipMap[skip] = struct{}{}
	}

	err := filepath.WalkDir(targetBase, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if isSkippedDir(path, skipMap) {
				verboseLogger.Printf("skipping directory %q", path)
				return fs.SkipDir
			}

			return nil
		}

		if isSkippedFile(targetBase, path) {
			verboseLogger.Printf("skipping file %q", path)
			return nil
		}

		_, ok := templates.TemplateFor(path)
		if !ok {
			// if there's no template for the given file, skip it
			return nil
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %q: %w", path, err)
		}

		targets = append(targets, target{
			path:     path,
			contents: string(contents),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return targets, nil
}

func isSkippedFile(base string, path string) bool {
	filename := filepath.Base(path)

	if filename == "go.mod" || filename == "go.sum" || filename == "go.work" || filename == "go.work.sum" {
		return true
	}

	if strings.HasPrefix(filename, "zz_generated") {
		return true
	}

	return false

}

func isSkippedDir(path string, allSkips map[string]struct{}) bool {
	_, shouldSkip := allSkips[filepath.Base(path)]

	return shouldSkip
}
