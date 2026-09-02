# The two free-text answers reach XML, JSON, Java and shell with no escaping

## Where it stands

`ProjectName` and `Description` are the only answers with no shape rule.
`ValidProjectName` requires non-blank and nothing else; `Description` is not
validated at all — `huh.NewInput()` with no `Validate` call, and no entry in
`Config.Validate`.

Both are then rendered by `text/template` — which escapes nothing — into five
different syntaxes:

| File | How it lands |
| --- | --- |
| `pom.xml.tmpl:10,12` | `<name>{{.ProjectName}}</name>`, `<description>{{.Description}}</description>` |
| `java/MainView.java.tmpl:43` | `@PageTitle("{{.ProjectName}}")` |
| `keycloak-realm.json.tmpl:12,19` | inside JSON string literals |
| `run.conf.tmpl:8` | `PROJECT_NAME="{{.ProjectName}}"`, sourced by `run.sh` |
| `README.md.tmpl` | prose, where nothing can go wrong |

Each of the first four has a character that breaks it, and they are ordinary
characters in a project description:

- `--description "R&D tooling"` → `&` in XML → `pom.xml` is not well-formed →
  Maven refuses to start. `&`, `<` and `>` are all live here.
- `--name 'The "Good" App'` → an unescaped `"` inside a Java string literal →
  `MainView.java` does not compile, and the error names a line the user never
  wrote.
- The same name in `realm.json` → invalid JSON → Keycloak fails to import the
  realm, and the symptom is a login that redirects nowhere.
- `--name '$(id) app'` → `run.conf` is *sourced* by `run.sh`, so the substitution
  runs on every task.

The tool's tests do not see any of it: `internal/generate/generate_test.go` uses
`ProjectName: "Note Harbor"` and `Description: "Somewhere to put things"` for
every one of its 32 combinations, so "the pom is well-formed XML" and "the realm
is valid JSON" are checked only against input that could not have broken them.

None of this is a security hole in the usual sense — the input is the user's own,
typed on their own machine, and `$(id)` in a project name is a foot-gun rather
than an attack. But "the generated project does not build, and the error blames a
file the tool wrote" is exactly the failure this tool exists to prevent.

## What to do

Escape at the point of rendering, per target syntax. `text/template` cannot do it
for you — `html/template` is for HTML and would be wrong here — so it is a small
set of template functions registered in `renderString`:

```go
template.FuncMap{
    "xml":   xmlEscape,   // & < > " '
    "json":  jsonString,  // encoding/json, minus the surrounding quotes
    "java":  javaString,  // " \ and the control characters
    "shell": shellDouble, // " $ ` \
}
```

and then `<name>{{.ProjectName | xml}}</name>`, `@PageTitle("{{.ProjectName |
java}}")`, and so on. Explicit at each use rather than applied to the value once:
the same string is correct in five different spellings, and a value that arrives
pre-escaped for XML would be wrong in the other four.

The alternative — narrowing `ValidProjectName` to a safe character class — is
worse. The name is prose that shows up in the UI and the page title, `&` and `'`
belong in it, and refusing them pushes the problem onto the user to work around.

## Test

This is the part worth doing first, because it fails today:

- Render every combination a second time with a deliberately hostile name and
  description — `Ampersand & "Quote" <Tag> $(id) \` — and run the existing
  assertions over it: the pom parses as XML, the realm parses as JSON, and every
  rendered `run.conf` survives `bash -n` with the value intact.
- A Java-side check is harder without a compiler. Asserting the rendered
  `@PageTitle` line contains no unescaped `"` is enough to catch the regression,
  and CI's `generated-project` job compiles the real thing.

Feeding the hostile fixture through the *existing* property tests is most of the
work; the escaping functions are the small part.
