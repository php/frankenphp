# {{.BaseName}} Extension

Auto-generated PHP extension from Go code.

{{if .Functions}}## Functions

{{range .Functions}}### {{.Name}}

```php
{{.Signature}}
```

{{if .Params}}**Parameters:**

{{range .Params}}- `{{.Name}}` ({{.PhpType}}){{if .IsNullable}} (nullable){{end}}{{if .HasDefault}} (default: {{.DefaultValue}}){{end}}
{{end}}
{{end}}**Returns:** {{.ReturnType}}{{if .IsReturnNullable}} (nullable){{end}}

{{end}}{{end}}{{if .Classes}}## Classes

{{range .Classes}}### {{.Name}}

{{if .Properties}}**Properties:**

{{range .Properties}}- `{{.Name}}`: {{.PhpType}}{{if .IsNullable}} (nullable){{end}}
{{end}}
{{end}}{{end}}{{end}}
