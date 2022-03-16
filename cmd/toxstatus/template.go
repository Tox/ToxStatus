package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

var (
	//go:embed assets/*
	assetsFs embed.FS

	templateMap = loadTemplates()
	funcMap     = template.FuncMap{
		"lower": strings.ToLower,
		"inc":   increment,
		"since": getTimeSinceString,
		"loc":   getLocString,
		"time":  getTimeString,
	}
)

func loadTemplates() (templates map[string]*template.Template) {
	templates = map[string]*template.Template{}
	baseBytes := mustGetAssetBytes("assets/templates/base.html")

	err := fs.WalkDir(assetsFs, "assets/templates/pages", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}
		if d.IsDir() {
			return nil
		}

		// parse the child layout
		tmplName := filepath.Base(path)
		bytes := mustGetAssetBytes(path)
		tmpl := template.Must(template.New(tmplName).Funcs(funcMap).Parse(string(bytes)))

		// and finally also parse the base layout
		templates[tmplName] = template.Must(tmpl.Parse(string(baseBytes)))

		return nil
	})
	if err != nil {
		panic(fmt.Errorf("unable to load page templates: %w", err))
	}

	return
}

func mustGetAssetBytes(name string) []byte {
	bytes, err := getAssetBytes(name)
	if err != nil {
		panic(err)
	}

	return bytes
}

func getAssetBytes(name string) ([]byte, error) {
	bytes, err := assetsFs.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("unable to read asset: %w", err)
	}

	return bytes, nil
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) error {
	tmpl, exists := templateMap[name]
	if !exists {
		return fmt.Errorf("template %s does not exist", name)
	}

	w.Header().Set("Content-Type", "text/html")
	return tmpl.ExecuteTemplate(w, "base", data)
}
