package main

import (
	"encoding/json"
	"fmt"
	// "log"
	"net/http"
	"net/url"
	"strings"
)

const aur_rpc_ver = 5

const aur_callback_max = 30

type AurPkg struct {
	Name           string   `json:"Name"`
	PackageBase    *string  `json:"PackageBase,omitempty"`
	Version        *string  `json:"Version,omitempty"`
	Depends        []string `json:"Depends"`
	MakeDepends    []string `json:"MakeDepends"`
	CheckDepends   []string `json:"CheckDepends"`
	OptDepends     []string `json:"OptDepends"`
	Provides       []string `json:"Provides"`
	Description    *string  `json:"Description,omitempty"`
	URL            *string  `json:"URL,omitempty"`
	URLPath        *string  `json:"URLPath,omitempty"`
	NumVotes       *int     `json:"NumVotes,omitempty"`
	Popularity     *float64 `json:"Popularity,omitempty"`
	OutOfDate      *int     `json:"OutOfDate,omitempty"`
	Maintainer     *string  `json:"Maintainer,omitempty"`
	Submitter      *string  `json:"Submitter,omitempty"`
	FirstSubmitted *int     `json:"FirstSubmitted,omitempty"`
	LastModified   *int     `json:"LastModified,omitempty"`
	ID             *int     `json:"ID,omitempty"`
	PackageBaseID  *int     `json:"PackageBaseID,omitempty"`
	Keywords       []string `json:"Keywords"`
	License        []string `json:"License"`
}

func fetchinfo(pkgnames []string) ([]AurPkg, error) {
	const aur_location = "https://aur.archlinux.org"
	aururl, err := url.Parse(aur_location)
	if err != nil {
		return nil, err
	}
	rpcpath := fmt.Sprintf("/rpc/v%d/info", aur_rpc_ver)
	aururl.Path = rpcpath

	params := url.Values{}
	for _, p := range pkgnames {
		params.Add("arg[]", p)
	}

	req, err := http.NewRequest(
		"POST",
		aururl.String(),
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "gaur")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var result struct {
		Results []AurPkg `json:"results"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return result.Results, nil

}

type DepType int

const (
	Depends DepType = iota
	MakeDepends
	CheckDepends
	OptDepends
	Self
)

type Dep struct {
	Spec string
	Type DepType
}

type ProviderInfo struct {
	Name    string
	Version string
}

func stripVersion(dep string) string {
	if i := strings.IndexAny(dep, "<>="); i != -1 {
		return dep[:i]
	}
	return dep
}

func allDeps(pkg AurPkg, types []DepType) []Dep {
	expandDepsFromType := func(deptype DepType) []string {
		switch deptype {
		case Depends:
			return pkg.Depends
		case MakeDepends:
			return pkg.MakeDepends
		case CheckDepends:
			return pkg.CheckDepends
		case OptDepends:
			return pkg.OptDepends
		default:
			panic("Invalid DepType")
		}

	}
	var deps []Dep
	for _, deptype := range types {
		for _, depname := range expandDepsFromType(deptype) {
			deps = append(deps, Dep{Spec: depname, Type: deptype})
		}
	}
	return deps
}

func recurse(pkgnames []string, types []DepType) (
	results map[string]AurPkg,
	pkgdeps map[string][]Dep,
	pkgmap map[string]ProviderInfo,
	err error) {

	results = make(map[string]AurPkg)
	pkgdeps = make(map[string][]Dep)
	pkgmap = make(map[string]ProviderInfo)
	tally := make(map[string]bool)

	var resolve func(depth int, batch []string) error

	resolve = func(depth int, batch []string) error {

		if depth >= aur_callback_max {
			return fmt.Errorf("Total requests: %d (out of range)", aur_callback_max)
		}
		if len(batch) == 0 {
			return nil
		}

		level, err := fetchinfo(batch)
		if err != nil {
			return err
		}

		var next []string
		for _, pkg := range level {
			// for the virtuals pkg provides
			for _, provSpec := range pkg.Provides {
				parts := strings.SplitN(provSpec, "=", 2)
				switch len(parts) {
				case 2:
					// pkg.Name = gcc provSpec cc=14 => pkgmap["cc"] = ProviderInfo{Name: "gcc", Version: "14"}
					pkgmap[parts[0]] = ProviderInfo{Name: pkg.Name, Version: parts[1]}
				case 1:
					pkgmap[parts[0]] = ProviderInfo{Name: pkg.Name, Version: ""}
				}
			}

			results[pkg.Name] = pkg
			tally[pkg.Name] = true

			for _, dep := range allDeps(pkg, types) {
				pkgdeps[pkg.Name] = append(pkgdeps[pkg.Name], dep)
				baredep := stripVersion(dep.Spec)
				if _, ok := tally[baredep]; !ok {
					tally[baredep] = true
					next = append(next, baredep)
				}
			}

		}
		return resolve(depth+1, next)
	}

	// Seed deps for self edges
	for _, p := range pkgnames {
		pkgdeps[p] = []Dep{{Spec: p, Type: Self}}

	}
	err = resolve(1, pkgnames)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(results) == 0 {
		return nil, nil, nil, fmt.Errorf("no packages found")
	}
	for _, name := range pkgnames {
		if _, ok := results[name]; !ok {
			fmt.Printf("target not found %s\n", name)
		}
	}

	return results, pkgdeps, pkgmap, nil
}

//
// func graph(
// 	provides bool,
// 	verify bool,
// 	results map[string]AurPkg,
// 	pkgdeps map[string]Dep,
// 	pkgmap map[string]ProviderInfo) {
//
// 	return
// }
//
// func solve(installed []string, verify bool, provides bool, types []DepType, targets []string) (
// 	results map[string]AurPkg,
// 	dag map[string]map[string]DepType,
// 	dag_foreign map[string]map[string]DepType) {
//
// 	results, pkgdeps, pkgmap := recurse(targets, types)
// 	dag, dag_foreign := graph(provides, verify, results, pkgdeps, pkgmap)
//
// 	return results, dag, dag_foreign
// }

func main() {

}
