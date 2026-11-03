package cli

import "regexp"

// handlerWire registers handlers.<Name>Handler(app) in main().
func handlerWire(module string, n Names) wireSpec {
	return wireSpec{
		Imports: []importSpec{{Path: module + "/handlers"}},
		Stmt:    func(app string) string { return "handlers." + n.Pascal + "Handler(" + app + ")" },
		Already: regexp.MustCompile(`handlers\.` + n.Pascal + `Handler\(`),
		After:   regexp.MustCompile(`^handlers\.\w+Handler\(`),
		Marker:  markerHandlers,
	}
}

// dbPluginWire registers the database plugin in main(). Each generated
// handler migrates its own model, so the plugin is opened without models.
func dbPluginWire() wireSpec {
	return wireSpec{
		Imports: []importSpec{{Path: aemeathModule + "/db"}, {Path: "log"}},
		Stmt: func(app string) string {
			return `if database, err := db.Open("app.db"); err != nil {
	log.Fatal(err)
} else if err := ` + app + `.RegisterPlugin(database); err != nil {
	log.Fatal(err)
}`
		},
		Already: regexp.MustCompile(`\bdb\.Open\(`),
		After:   regexp.MustCompile(`RegisterPlugin\(`),
		Marker:  markerPlugins,
	}
}

// jobsRunnerWire registers the background job runner in main(). The framework's
// package is imported as aejobs because a project's own package is usually named jobs.
func jobsRunnerWire() wireSpec {
	return wireSpec{
		Imports: []importSpec{{Alias: "aejobs", Path: aemeathModule + "/jobs"}, {Path: "log"}},
		Stmt: func(app string) string {
			return `if err := ` + app + `.RegisterPlugin(aejobs.New(aejobs.WithWorkers(4))); err != nil {
	log.Fatal(err)
}`
		},
		Already: regexp.MustCompile(`\baejobs\.New\(|aemeath/jobs"`),
		After:   regexp.MustCompile(`RegisterPlugin\(`),
		Marker:  markerPlugins,
	}
}
