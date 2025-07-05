<?php

/** @generate-class-entries */
{{if .Namespace}}
namespace {{.Namespace}};
{{end}}
{{range .Constants}}{{if eq .ClassName ""}}{{if .IsIota}}/**
 * @var int
 * @cvalue {{.Name}}
 */
const {{.Name}} = UNKNOWN;

{{else}}/**
 * @var {{phpType .PhpType}}
 */
const {{.Name}} = {{.Value}};

{{end}}{{end}}{{end}}{{range .Functions}}function {{.Signature}} {}

{{end}}{{range .Classes}}{{$className := .Name}}class {{.Name}} {
{{range $.Constants}}{{if eq .ClassName $className}}{{if .IsIota}}    /**
     * @var int
     * @cvalue {{.Name}}
     */
    public const {{.Name}} = UNKNOWN;

{{else}}    /**
     * @var {{phpType .PhpType}}
     */
    public const {{.Name}} = {{.Value}};

{{end}}{{end}}{{end}}
    public function __construct() {}
{{range .Methods}}
    public function {{.Signature}} {}
{{end}}
}

{{end}}
