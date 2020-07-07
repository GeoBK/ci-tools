package backporter

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/openshift/ci-tools/pkg/httphelper"

	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/test-infra/prow/bugzilla"
)

const (
	// BugIDQuery stores the query for bug ID
	BugIDQuery = "ID"
)

const htmlPageStart = `
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8"><title>%s</title>
<link rel="stylesheet" href="https://stackpath.bootstrapcdn.com/bootstrap/4.1.3/css/bootstrap.min.css" integrity="sha384-MCw98/SFnGE8fJT3GXwEOngsV7Zt27NXFoaoApmYm81iuXoPkFOJwJ8ERdknLPMO" crossorigin="anonymous">
<script src="https://code.jquery.com/jquery-3.3.1.slim.min.js" integrity="sha384-q8i/X+965DzO0rT7abK41JStQIAqVgRVzpbzo5smXKp4YfRvH+8abtTE1Pi6jizo" crossorigin="anonymous"></script>
<script src="https://stackpath.bootstrapcdn.com/bootstrap/4.1.3/js/bootstrap.min.js" integrity="sha384-ChfqqxuZUCnJSK3+MXmPNIyE6ZbWh2IMqE241rYiqJxyMiZ6OW/JmZQ5stwEULTy" crossorigin="anonymous"></script>
<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
<style>
@namespace svg url(http://www.w3.org/2000/svg);
svg|a:link, svg|a:visited {
  cursor: pointer;
}

svg|a text,
text svg|a {
  fill: #007bff;
  text-decoration: none;
  background-color: transparent;
  -webkit-text-decoration-skip: objects;
}
select:invalid { color: gray; }
svg|a:hover text, svg|a:active text {
  fill: #0056b3;
  text-decoration: underline;
}

pre {
	border: 10px solid transparent;
}
h1, h2, p {
	padding-top: 10px;
}
h1 a:link,
h2 a:link,
h3 a:link,
h4 a:link,
h5 a:link {
  color: inherit;
  text-decoration: none;
}
h1 a:hover,
h2 a:hover,
h3 a:hover,
h4 a:hover,
h5 a:hover {
  text-decoration: underline;
}
h1 a:visited,
h2 a:visited,
h3 a:visited,
h4 a:visited,
h5 a:visited {
  color: inherit;
  text-decoration: none;
}
.info {
	text-decoration-line: underline;
	text-decoration-style: dotted;
	text-decoration-color: #c0c0c0;
}
button {
  padding:0.2em 1em;
  border-radius: 8px;
  cursor:pointer;
}
td {
  vertical-align: middle;
}
</style>
</head>
<body>
<nav class="navbar navbar-expand-lg navbar-light bg-light">
  <a class="navbar-brand" href="/">Bugzilla Backporter</a>
  <button class="navbar-toggler" type="button" data-toggle="collapse" data-target="#navbarSupportedContent" aria-controls="navbarSupportedContent" aria-expanded="false" aria-label="Toggle navigation">
    <span class="navbar-toggler-icon"></span>
  </button>

  <div class="collapse navbar-collapse" id="navbarSupportedContent">
    <form class="form-inline my-2 my-lg-0 needs-validation" role="search" action="/clones" method="get">
	  <input class="form-control mr-sm-2" type="text" placeholder="Bug ID" aria-label="Search" name="ID" required>
      <button class="btn btn-outline-success my-2 my-sm-0" type="submit">Find Clones</button>
    </form>
  </div>
</nav>
`

const htmlPageEnd = `
<footer>
<p class="small">Source code for this page located on <a href="https://github.com/openshift/ci-tools">GitHub</a></p>
</footer>
</body>
</html>
`

const clonesTemplateConstructor = `
{{if .NewCloneID }}
	<div class="alert alert-success alert-dismissible" id="success-banner">
	<a href="#" class="close" data-dismiss="alert" aria-label="close">&times;</a>
	<strong>Success!</strong> Clone created - <a href="/clones?ID={{ .NewCloneID }}" >Bug#{{ .NewCloneID }}</a>.
	</div>
{{ end }}
<div class="container">
	<h2> <a href = "#bugid" id="bugid" value="{{.Bug.ID}}"> {{.Bug.ID}}: {{.Bug.Summary}} </a> | Status: {{.Bug.Status}} </h2>
	<p><label> Target Release:</label> {{ .Bug.TargetRelease }} </p>
	{{ if .PRs }}
		<p> <label>GitHub PR: </label>
		{{ if .PRs }}
			{{ range $index, $pr := .PRs }}
				{{ if $index}}|{{end}} 
				<a href="{{ $pr.Type.URL }}/{{ $pr.Org}}/{{ $pr.Repo}}/pull/{{ $pr.Num}}">{{ $pr.Org}}/{{ $pr.Repo}}#{{ $pr.Num}}</a>
			{{ end }}
		{{ end }}
		</p>
	{{ else }}
		<p> No linked PRs. </p>
	{{ end }}
	{{ if ne .Parent.ID .Bug.ID}}
		<p> <label>Cloned From: </label><a href = "/clones?ID={{.Parent.ID}}"> Bug {{.Parent.ID}}: {{.Parent.Summary}}</a> | Status: {{.Parent.Status}}
	{{ else }}
		<p> <label>Cloned From: </label>This is the original. </p>
	{{ end }}
	<h4 id="clones"> <a href ="#clones"> Clones</a> </h4>
	<table class="table">
		<thead>
			<tr>
				<th title="Targeted version to release fix" class="info">Target Release</th>
				<th title="ID of the cloned bug" class="info">Bug ID</th>
				<th title="Status of the cloned bug" class="info">Status</th>
				<th title="PR associated with this bug" class="info">PR</th>
			</tr>
		</thead>
		<tbody>
		{{ if .Clones }}
			{{ range $clone := .Clones }}
				<tr>
					<td style="vertical-align: middle;">{{ $clone.TargetRelease }}</td>
					<td style="vertical-align: middle;"><a href = "/clones?ID={{$clone.ID}}">{{ $clone.ID }}</a></td>
					<td style="vertical-align: middle;">{{ $clone.Status }}</td>
					<td style="vertical-align: middle;">
						{{range $index, $pr := $clone.PRs }}
							{{ if $index}},{{end}}
							<a href = "{{ $pr.Type.URL }}/{{$pr.Org}}/{{$pr.Repo}}/pull/{{$pr.Num}}" target="_blank"> {{$pr.Org}}/{{$pr.Repo}}#{{$pr.Num}}</a>
						{{end}}
					</td>
				</tr>
			{{ end }}
		{{ else }}
			<tr> <td colspan=4 style="text-align:center;"> No clones found. </td></tr>
		{{ end }}
		</tbody>
	</table>
	<form class="form-inline my-2 my-lg-0" role="search" action="/clones/create" method="post">
		<input type="hidden" name="ID" value="{{.Bug.ID}}">
		<select class="form-control mr-sm-2" aria-label="Search" name="release" id="target_version" required>
			<option value="" disabled selected hidden>Target Version</option>
			{{ range $release := .CloneTargets }}
				<option value="{{$release}}" id="opt_{{$release}}">{{$release}}</option>
			{{end}}
		</select>
		<button class="btn btn-outline-success my-2 my-sm-0" type="submit">Create Clone</button>
    </form>
</div>`

const errorTemplateConstructor = `
<div class="alert alert-danger" id="error-banner">
<a href="#" class="close" data-dismiss="alert" aria-label="close">&times;</a>
<strong>Error </strong> <label id ="error-text">{{.}}</label>
</div>`

var (
	clonesTemplate = template.Must(template.New("clones").Parse(clonesTemplateConstructor))
	emptyTemplate  = template.Must(template.New("empty").Parse("{{.}}"))
	errorTemplate  = template.Must(template.New("error").Parse(errorTemplateConstructor))
)

func logFieldsFor(endpoint string, bugID int) logrus.Fields {
	return logrus.Fields{
		"endpoint": endpoint,
		"bugID":    bugID,
	}
}

func handleError(w http.ResponseWriter, err error, shortErrorMessage string, statusCode int, endpoint string, bugID int, metrics *httphelper.Metrics) {
	var fprintfErr error
	w.WriteHeader(statusCode)
	wpErr := writePage(w, http.StatusText(statusCode), errorTemplate, shortErrorMessage)
	if wpErr != nil {
		_, fprintfErr = fmt.Fprintf(w, "failed while building error page")
	}
	metrics.RecordError(shortErrorMessage)
	logrus.WithFields(logFieldsFor(endpoint, bugID)).WithError(fmt.Errorf("%s: %v", shortErrorMessage, utilerrors.NewAggregate([]error{err, wpErr, fprintfErr})))
}

// HandlerFuncWithErrorReturn allows returning errors to be logged
type HandlerFuncWithErrorReturn func(http.ResponseWriter, *http.Request) error

// ClonesTemplateData holds the UI data for the clones page
type ClonesTemplateData struct {
	Bug          *bugzilla.Bug          // bug details
	Clones       []*bugzilla.Bug        // List of clones for the bug
	Parent       *bugzilla.Bug          // Root bug if it is a a bug, otherwise holds itself
	PRs          []bugzilla.ExternalBug // Details of linked PR
	CloneTargets []string
	NewCloneID   int
}

// Writes an HTML page, prepends header in htmlPageStart and appends header from htmlPageEnd around tConstructor.
func writePage(w http.ResponseWriter, title string, body *template.Template, data interface{}) error {
	_, err := fmt.Fprintf(w, htmlPageStart, title)
	if err != nil {
		return err
	}
	if err := body.Execute(w, data); err != nil {
		return err
	}
	_, fprintfErr := fmt.Fprint(w, htmlPageEnd)
	if fprintfErr != nil {
		return err
	}
	return nil
}

// GetLandingHandler will return a simple bug search page
func GetLandingHandler(metrics *httphelper.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		err := writePage(w, "Home", emptyTemplate, nil)
		if err != nil {
			handleError(w, err, "failed to build Landing page", http.StatusInternalServerError, req.URL.Path, 0, metrics)
		}
	}
}

// GetBugHandler returns a function with bug details  in JSON format
func GetBugHandler(client bugzilla.Client, metrics *httphelper.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		endpoint := r.URL.Path
		if r.Method != "GET" {
			http.Error(w, "not a valid request method: expected GET", http.StatusBadRequest)
			metrics.RecordError("not a valid request method: expected GET")
			logrus.WithFields(logFieldsFor(endpoint, 0)).WithError(fmt.Errorf("not a valid request method: expected GET"))
			return
		}
		bugIDStr := r.URL.Query().Get(BugIDQuery)
		if bugIDStr == "" {
			http.Error(w, "missing mandatory query arg: \"ID\"", http.StatusBadRequest)
			metrics.RecordError("missing mandatory query arg: \"ID\"")
			logrus.WithFields(logFieldsFor(endpoint, 0)).WithError(fmt.Errorf("missing mandatory query arg: \"ID\""))
			return
		}
		bugID, err := strconv.Atoi(bugIDStr)
		if err != nil {
			http.Error(w, "unable to convert \"ID\" from string to int", http.StatusBadRequest)
			metrics.RecordError("unable to convert \"ID\" from string to int")
			logrus.WithFields(logFieldsFor(endpoint, 0)).WithError(fmt.Errorf("unable to convert \"ID\" from string to int"))
			return
		}

		bugInfo, err := client.GetBug(bugID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Bug#%d not found", bugID), http.StatusNotFound)
			metrics.RecordError("BugID not found")
			logrus.WithFields(logFieldsFor(endpoint, bugID)).WithError(fmt.Errorf("Bug#%d not found: %w", bugID, err))
			return
		}

		jsonBugInfo, err := json.MarshalIndent(*bugInfo, "", "  ")
		if err != nil {
			http.Error(w, "failed to marshal bugInfo to JSON", http.StatusInternalServerError)
			metrics.RecordError("failed to marshal bugInfo to JSON")
			logrus.WithFields(logFieldsFor(endpoint, bugID)).WithError(fmt.Errorf("failed to marshal bugInfo to JSON: %w", err))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(jsonBugInfo)
		if err != nil {
			http.Error(w, "unable to write to responsewriter for getBugHandler", http.StatusInternalServerError)
			metrics.RecordError("unable to write to responsewriter")
			logrus.WithFields(logFieldsFor(endpoint, bugID)).WithError(fmt.Errorf("unable to write to responsewriter for getBugHandler: %w", err))
			return
		}
	}
}

func getClonesTemplateData(bugID int, client bugzilla.Client, allTargetVersions sets.String) (*ClonesTemplateData, int, error) {
	bug, err := client.GetBug(bugID)
	if err != nil {
		return nil, http.StatusNotFound, fmt.Errorf("Bug#%d not found: %w", bugID, err)
	}

	prs, err := client.GetExternalBugPRsOnBug(bug.ID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("Bug#%d - error occured while retreiving list of PRs: %w", bug.ID, err)
	}

	parent, err := client.GetRootForClone(bug)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("unable to fetch root bug: %w", err)
	}

	clones, err := client.GetAllClones(bug)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("unable to get clones: %w", err)
	}
	// Target versions would be used to populate the CreateClone dropdown
	targetVersions := sets.NewString(allTargetVersions.List()...)
	// Remove target versions of the original bug
	targetVersions.Delete(bug.TargetRelease...)
	for _, clone := range clones {
		clonePRs, err := client.GetExternalBugPRsOnBug(clone.ID)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("Bug#%d - error occured while retreiving list of PRs : %w", clone.ID, err)
		}

		// Remove target releases which already have clones
		targetVersions.Delete(clone.TargetRelease...)

		clone.PRs = clonePRs
	}
	wrpr := ClonesTemplateData{
		Bug:          bug,
		Clones:       clones,
		Parent:       parent,
		PRs:          prs,
		CloneTargets: targetVersions.List(),
	}
	return &wrpr, http.StatusOK, nil
}

// GetClonesHandler returns an HTML page with detais about the bug and its clones
func GetClonesHandler(client bugzilla.Client, allTargetVersions sets.String, metrics *httphelper.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" {
			handleError(w, fmt.Errorf("invalid request method, expected GET got %s", req.Method), "invalid request method", http.StatusBadRequest, req.URL.Path, 0, metrics)
			return
		}

		bugIDStr := req.URL.Query().Get(BugIDQuery)
		if bugIDStr == "" {
			handleError(w, fmt.Errorf("missing mandatory query arg: \"ID\""), "missing mandatory query arg: \"ID\"", http.StatusBadRequest, req.URL.Path, 0, metrics)
			return
		}
		bugID, err := strconv.Atoi(bugIDStr)
		if err != nil {
			handleError(w, err, "unable to convert \"ID\" from string to int", http.StatusBadRequest, req.URL.Path, 0, metrics)
			return
		}

		wrpr, statusCode, err := getClonesTemplateData(bugID, client, allTargetVersions)
		if err != nil {
			handleError(w, err, "unable to get get bug details", statusCode, req.URL.Path, bugID, metrics)
			return
		}
		err = writePage(w, "Clones", clonesTemplate, wrpr)
		if err != nil {
			handleError(w, err, "failed to build Clones page", http.StatusInternalServerError, req.URL.Path, bugID, metrics)
		}
	}
}

// CreateCloneHandler will create a clone of the specified ID and return success/error
func CreateCloneHandler(client bugzilla.Client, allTargetVersions sets.String, metrics *httphelper.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		endpoint := req.URL.Path
		if req.Method != "POST" {
			handleError(w, fmt.Errorf("invalid request method, expected POST got %s", req.Method), "invalid request method", http.StatusBadRequest, req.URL.Path, 0, metrics)
			return
		}
		// Parse the parameters passed in the POST request
		err := req.ParseForm()
		if err != nil {
			handleError(w, err, "unable to parse request", http.StatusBadRequest, req.URL.Path, 0, metrics)
			return
		}
		if req.FormValue("ID") == "" {
			handleError(w, fmt.Errorf("missing mandatory query arg: \"ID\""), "missing mandatory query arg: \"ID\"", http.StatusBadRequest, req.URL.Path, 0, metrics)
			return
		}
		bugID, err := strconv.Atoi(req.FormValue("ID"))
		if err != nil {
			handleError(w, err, fmt.Sprintf("unable to convert \"ID\" parameter from string to int: %s", req.FormValue("ID")), http.StatusBadRequest, req.URL.Path, 0, metrics)
			return
		}
		// Get the details of the bug
		bug, err := client.GetBug(bugID)
		if err != nil {
			handleError(w, err, fmt.Sprintf("unable to fetch bug details- Bug#%d", bugID), http.StatusNotFound, endpoint, bugID, metrics)
			return
		}
		// Create a clone of the bug
		cloneID, err := client.CloneBug(bug)
		if err != nil {
			handleError(w, err, "clone creation failed", http.StatusInternalServerError, endpoint, bugID, metrics)
			return
		}
		targetRelease := bugzilla.BugUpdate{
			TargetRelease: []string{
				req.FormValue("release"),
			},
		}
		// Updating the cloned bug with the right target version
		if err := client.UpdateBug(cloneID, targetRelease); err != nil {
			handleError(w, err, fmt.Sprintf("clone created - Bug#%d, but failed to specify version for the cloned bug.", cloneID), http.StatusInternalServerError, endpoint, bugID, metrics)
			return
		}
		// Repopulate the fields of the page with the right data
		data, statusCode, err := getClonesTemplateData(bugID, client, allTargetVersions)
		if err != nil {
			handleError(w, err, "unable to get get bug details", statusCode, endpoint, bugID, metrics)
			return
		}
		// Populating the NewCloneId which is used to show the success info banner
		data.NewCloneID = cloneID
		err = writePage(w, "Clones", clonesTemplate, *data)
		if err != nil {
			handleError(w, err, "failed to build CreateClones response page", http.StatusInternalServerError, req.URL.Path, bugID, metrics)
		}
	}
}
