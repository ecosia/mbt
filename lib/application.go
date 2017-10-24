package lib

import (
	"container/list"
	"fmt"
	"sort"
	"strings"

	"github.com/buddyspike/graph"
	git "github.com/libgit2/git2go"
)

// Application represents a single application in the repository.
type Application struct {
	name       string
	path       string
	build      map[string]*BuildCmd
	hash       string
	version    string
	properties map[string]interface{}
	requires   *list.List
	requiredBy *list.List
}

// Applications is an array of Application.
type Applications []*Application

// Name returns the name of the application.
func (a *Application) Name() string {
	return a.name
}

// Path returns the relative path to application.
func (a *Application) Path() string {
	return a.path
}

// Build returns the build configuration for the application.
func (a *Application) Build() map[string]*BuildCmd {
	return a.build
}

// Properties returns the custom properties in the configuration.
func (a *Application) Properties() map[string]interface{} {
	return a.properties
}

// Requires returns an array of applications required by this application.
func (a *Application) Requires() *list.List {
	return a.requires
}

// RequiredBy returns an array of applications requires this application.
func (a *Application) RequiredBy() *list.List {
	return a.requiredBy
}

// Version returns the content based version SHA for the application.
func (a *Application) Version() string {
	return a.version
}

// Sort interface to sort applications by path
func (l Applications) Len() int {
	return len(l)
}

func (l Applications) Less(i, j int) bool {
	return l[i].path < l[j].path
}

func (l Applications) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

type requiredByNodeProvider struct{}

func (p *requiredByNodeProvider) ID(vertex interface{}) interface{} {
	return vertex.(*Application).Name()
}

func (p *requiredByNodeProvider) ChildCount(vertex interface{}) int {
	return vertex.(*Application).RequiredBy().Len()
}

func (p *requiredByNodeProvider) Child(vertex interface{}, index int) (interface{}, error) {
	head := vertex.(*Application).RequiredBy().Front()
	for i := 0; i < index; i++ {
		head = head.Next()
	}

	return head.Value, nil
}

type requiresNodeProvider struct{}

func (p *requiresNodeProvider) ID(vertex interface{}) interface{} {
	return vertex.(*Application).Name()
}

func (p *requiresNodeProvider) ChildCount(vertex interface{}) int {
	return vertex.(*Application).Requires().Len()
}

func (p *requiresNodeProvider) Child(vertex interface{}, index int) (interface{}, error) {
	head := vertex.(*Application).Requires().Front()
	for i := 0; i < index; i++ {
		head = head.Next()
	}

	return head.Value, nil
}

func newApplication(metadata *applicationMetadata, requires *list.List) *Application {
	spec := metadata.spec
	app := &Application{
		build:      spec.Build,
		name:       spec.Name,
		properties: spec.Properties,
		hash:       metadata.hash,
		path:       metadata.dir,
		requires:   new(list.List),
		requiredBy: new(list.List),
	}

	if requires != nil {
		app.requires.PushBackList(requires)
	}

	return app
}

func (l Applications) indexByName() map[string]*Application {
	q := make(map[string]*Application)
	for _, a := range l {
		q[a.Name()] = a
	}
	return q
}

func (l Applications) indexByPath() map[string]*Application {
	q := make(map[string]*Application)
	for _, a := range l {
		q[fmt.Sprintf("%s/", a.Path())] = a
	}
	return q
}

// expandRequiredByDependencies takes a list of Applications and
// returns a new list of Applications including the ones in their
// requiredBy (see below) dependency chain.
// requiredBy dependency
// Application dependencies are described in two forms requires and requiredBy.
// If A needs B, then, A requires B and B is requiredBy A.
func (l Applications) expandRequiredByDependencies() (Applications, error) {
	// Step 1
	// Create the new list with all nodes
	g := new(list.List)
	for _, a := range l {
		g.PushBack(a)
	}

	// Step 2
	// Top sort it by requiredBy chain.
	allItems, err := graph.TopSort(g, &requiredByNodeProvider{})
	if err != nil {
		return nil, wrap(err)
	}

	// Step 3
	// Copy resulting array in the reverse order
	// because we top sorted by requiredBy chain.
	r := make([]*Application, allItems.Len())
	i := allItems.Len() - 1
	for e := allItems.Front(); e != nil; e = e.Next() {
		r[i] = e.Value.(*Application)
		i--
	}

	return r, nil
}

func applicationsInCommit(repo *git.Repository, commit *git.Commit) (Applications, error) {
	metadataSet, err := discoverMetadata(repo, commit)
	if err != nil {
		return nil, err
	}

	vapps, err := metadataSet.toApplications(true)
	if err != nil {
		return nil, err
	}

	sort.Sort(vapps)
	return vapps, nil
}

func applicationsInDiff(repo *git.Repository, to, from *git.Commit) (Applications, error) {
	diff, err := getDiffFromMergeBase(repo, to, from)
	if err != nil {
		return nil, err
	}

	a, err := applicationsInCommit(repo, to)
	if err != nil {
		return nil, err
	}

	return reduceToDiff(a, diff)
}

func reduceToDiff(applications Applications, diff *git.Diff) (Applications, error) {
	q := applications.indexByPath()
	filtered := make(map[string]*Application)
	err := diff.ForEach(func(delta git.DiffDelta, num float64) (git.DiffForEachHunkCallback, error) {
		for k := range q {
			if _, ok := filtered[k]; ok {
				continue
			}
			if strings.HasPrefix(delta.NewFile.Path, k) {
				filtered[k] = q[k]
			}
		}
		return nil, nil
	}, git.DiffDetailFiles)

	if err != nil {
		return nil, wrap(err)
	}

	apps := Applications{}
	for _, v := range filtered {
		apps = append(apps, v)
	}

	expandedApps, err := apps.expandRequiredByDependencies()
	if err != nil {
		return nil, err
	}

	return expandedApps, nil
}