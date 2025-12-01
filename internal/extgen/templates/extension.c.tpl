#include <php.h>
#include <Zend/zend_API.h>
#include <Zend/zend_hash.h>
#include <Zend/zend_types.h>
#include <stddef.h>

#include "{{.BaseName}}.h"
#include "{{.BaseName}}_arginfo.h"
#include "_cgo_export.h"

{{- if .Classes}}

#define VALIDATE_GO_HANDLE(intern) \
    do { \
        if ((intern)->go_handle == 0) { \
            zend_throw_error(NULL, "Go object not found in registry"); \
            RETURN_THROWS(); \
        } \
    } while (0)

static zend_object_handlers object_handlers_{{.BaseName}};

typedef struct {
    uintptr_t go_handle;
    zend_object std; /* This must be the last field in the structure: the property store starts at this offset */
} {{.BaseName}}_object;

static inline {{.BaseName}}_object *{{.BaseName}}_object_from_obj(zend_object *obj) {
    return ({{.BaseName}}_object*)((char*)(obj) - offsetof({{.BaseName}}_object, std));
}

static zend_object *{{.BaseName}}_create_object(zend_class_entry *ce) {
    {{.BaseName}}_object *intern = ecalloc(1, sizeof({{.BaseName}}_object) + zend_object_properties_size(ce));
    
    zend_object_std_init(&intern->std, ce);
    object_properties_init(&intern->std, ce);
    
    intern->std.handlers = &object_handlers_{{.BaseName}};
    intern->go_handle = 0; /* will be set in __construct */

    return &intern->std;
}

static void {{.BaseName}}_free_object(zend_object *object) {
    {{.BaseName}}_object *intern = {{.BaseName}}_object_from_obj(object);

    if (intern->go_handle != 0) {
        removeGoObject(intern->go_handle);
    }
    
    zend_object_std_dtor(&intern->std);
}

void init_object_handlers() {
    memcpy(&object_handlers_{{.BaseName}}, &std_object_handlers, sizeof(zend_object_handlers));
    object_handlers_{{.BaseName}}.free_obj = {{.BaseName}}_free_object;
    object_handlers_{{.BaseName}}.clone_obj = NULL;
    object_handlers_{{.BaseName}}.offset = offsetof({{.BaseName}}_object, std);
}
{{- end}}
{{ range .Classes}}
static zend_class_entry *{{.Name}}_ce = NULL;

PHP_METHOD({{namespacedClassName $.Namespace .Name}}, __construct) {
    ZEND_PARSE_PARAMETERS_NONE();

    {{$.BaseName}}_object *intern = {{$.BaseName}}_object_from_obj(Z_OBJ_P(ZEND_THIS));

    /* Constructor is called more than once, make it no-op */
    if (intern->go_handle != 0) {
        return;
    }

    intern->go_handle = create_{{.GoStruct}}_object();
}

{{ range .Methods}}
PHP_METHOD({{namespacedClassName $.Namespace .ClassName}}, {{.PhpName}}) {
    {{$.BaseName}}_object *intern = {{$.BaseName}}_object_from_obj(Z_OBJ_P(ZEND_THIS));
    
    VALIDATE_GO_HANDLE(intern);
    
    {{- if .Params -}}
    {{range $i, $param := .Params -}}
    {{- if eq $param.PhpType "string"}}
    zend_string *{{$param.Name}} = NULL;{{if $param.IsNullable}}
    zend_bool {{$param.Name}}_is_null = 0;{{end}}
    {{- else if eq $param.PhpType "int"}}
    zend_long {{$param.Name}} = {{if $param.HasDefault}}{{$param.DefaultValue}}{{else}}0{{end}};{{if $param.IsNullable}}
    zend_bool {{$param.Name}}_is_null = 0;{{end}}
    {{- else if eq $param.PhpType "float"}}
    double {{$param.Name}} = {{if $param.HasDefault}}{{$param.DefaultValue}}{{else}}0.0{{end}};{{if $param.IsNullable}}
    zend_bool {{$param.Name}}_is_null = 0;{{end}}
    {{- else if eq $param.PhpType "bool"}}
    zend_bool {{$param.Name}} = {{if $param.HasDefault}}{{if eq $param.DefaultValue "true"}}1{{else}}0{{end}}{{else}}0{{end}};{{if $param.IsNullable}}
    zend_bool {{$param.Name}}_is_null = 0;{{end}}
    {{- else if eq $param.PhpType "array"}}
    zval *{{$param.Name}} = NULL;
    {{- end}}
    {{- end}}
    
    {{$requiredCount := 0}}{{range .Params}}{{if not .HasDefault}}{{$requiredCount = add1 $requiredCount}}{{end}}{{end -}}
    ZEND_PARSE_PARAMETERS_START({{$requiredCount}}, {{len .Params}})
        {{$optionalStarted := false}}{{range .Params}}{{if .HasDefault}}{{if not $optionalStarted -}}
        Z_PARAM_OPTIONAL
        {{$optionalStarted = true}}{{end}}{{end -}}
        {{if .IsNullable}}{{if eq .PhpType "string"}}Z_PARAM_STR_OR_NULL({{.Name}}, {{.Name}}_is_null){{else if eq .PhpType "int"}}Z_PARAM_LONG_OR_NULL({{.Name}}, {{.Name}}_is_null){{else if eq .PhpType "float"}}Z_PARAM_DOUBLE_OR_NULL({{.Name}}, {{.Name}}_is_null){{else if eq .PhpType "bool"}}Z_PARAM_BOOL_OR_NULL({{.Name}}, {{.Name}}_is_null){{else if eq .PhpType "array"}}Z_PARAM_ARRAY_OR_NULL({{.Name}}){{end}}{{else}}{{if eq .PhpType "string"}}Z_PARAM_STR({{.Name}}){{else if eq .PhpType "int"}}Z_PARAM_LONG({{.Name}}){{else if eq .PhpType "float"}}Z_PARAM_DOUBLE({{.Name}}){{else if eq .PhpType "bool"}}Z_PARAM_BOOL({{.Name}}){{else if eq .PhpType "array"}}Z_PARAM_ARRAY({{.Name}}){{end}}{{end}}
        {{end -}}
    ZEND_PARSE_PARAMETERS_END();
    {{else}}
    ZEND_PARSE_PARAMETERS_NONE();
    {{end}}
    
    {{- if ne .ReturnType "void"}}
    {{- if eq .ReturnType "string"}}
    zend_string* result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .PhpType "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .PhpType "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "array"}}{{.Name}}{{end}}{{else}}{{.Name}}{{end}}{{end}}{{end}});
    if (result) {
        RETURN_STR(result);
    }
    RETURN_EMPTY_STRING();
    {{- else if eq .ReturnType "int"}}
    zend_long result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .PhpType "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .PhpType "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "array"}}{{.Name}}{{end}}{{else}}{{if eq .PhpType "array"}}{{.Name}}{{else}}(long){{.Name}}{{end}}{{end}}{{end}}{{end}});
    RETURN_LONG(result);
    {{- else if eq .ReturnType "float"}}
    double result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .PhpType "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .PhpType "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "array"}}{{.Name}}{{end}}{{else}}{{if eq .PhpType "array"}}{{.Name}}{{else}}(double){{.Name}}{{end}}{{end}}{{end}}{{end}});
    RETURN_DOUBLE(result);
    {{- else if eq .ReturnType "bool"}}
    int result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .PhpType "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .PhpType "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "array"}}{{.Name}}{{end}}{{else}}{{if eq .PhpType "array"}}{{.Name}}{{else}}(int){{.Name}}{{end}}{{end}}{{end}}{{end}});
    RETURN_BOOL(result);
    {{- else if eq .ReturnType "array"}}
    void* result = {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .PhpType "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .PhpType "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "array"}}{{.Name}}{{end}}{{else}}{{.Name}}{{end}}{{end}}{{end}});
    if (result != NULL) {
        HashTable *ht = (HashTable*)result;
        RETURN_ARR(ht);
    } else {
        RETURN_NULL();
    }
    {{- end}}
    {{- else}}
    {{.Name}}_wrapper(intern->go_handle{{if .Params}}{{range .Params}}, {{if .IsNullable}}{{if eq .PhpType "string"}}{{.Name}}_is_null ? NULL : {{.Name}}{{else if eq .PhpType "int"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "float"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "bool"}}{{.Name}}_is_null ? NULL : &{{.Name}}{{else if eq .PhpType "array"}}{{.Name}}{{end}}{{else}}{{if eq .PhpType "string"}}{{.Name}}{{else if eq .PhpType "int"}}(long){{.Name}}{{else if eq .PhpType "float"}}(double){{.Name}}{{else if eq .PhpType "bool"}}(int){{.Name}}{{else if eq .PhpType "array"}}{{.Name}}{{end}}{{end}}{{end}}{{end}});
    {{- end}}
}
{{end}}{{end}}

{{- if .Classes}}
void register_all_classes() {
    init_object_handlers();
    
    {{- range .Classes}}
    {{.Name}}_ce = register_class_{{namespacedClassName $.Namespace .Name}}();
    if (!{{.Name}}_ce) {
        php_error_docref(NULL, E_ERROR, "Failed to register class {{.Name}}");
        return;
    }
    {{.Name}}_ce->create_object = {{$.BaseName}}_create_object;
    {{- end}}
}
{{- end}}

PHP_MINIT_FUNCTION({{.BaseName}}) {
    {{ if .Classes}}register_all_classes();{{end}}

    {{- range .Constants}}
    {{- if eq .ClassName ""}}
    {{- if $.Namespace}}
        {{if .IsIota}}REGISTER_NS_LONG_CONSTANT("{{cString $.Namespace}}", "{{.Name}}", {{.Name}}, CONST_CS | CONST_PERSISTENT);
        {{else if eq .PhpType "string"}}REGISTER_NS_STRING_CONSTANT("{{cString $.Namespace}}", "{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
        {{else if eq .PhpType "bool"}}REGISTER_NS_LONG_CONSTANT("{{cString $.Namespace}}", "{{.Name}}", {{if eq .Value "true"}}1{{else}}0{{end}}, CONST_CS | CONST_PERSISTENT);
        {{else if eq .PhpType "float"}}REGISTER_NS_DOUBLE_CONSTANT("{{cString $.Namespace}}", "{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
        {{else}}REGISTER_NS_LONG_CONSTANT("{{cString $.Namespace}}", "{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
        {{- end}}
    {{- else}}
    {{if .IsIota}}REGISTER_LONG_CONSTANT("{{.Name}}", {{.Name}}, CONST_CS | CONST_PERSISTENT);
    {{else if eq .PhpType "string"}}REGISTER_STRING_CONSTANT("{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
    {{else if eq .PhpType "bool"}}REGISTER_LONG_CONSTANT("{{.Name}}", {{if eq .Value "true"}}1{{else}}0{{end}}, CONST_CS | CONST_PERSISTENT);
    {{else if eq .PhpType "float"}}REGISTER_DOUBLE_CONSTANT("{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
    {{else}}REGISTER_LONG_CONSTANT("{{.Name}}", {{.CValue}}, CONST_CS | CONST_PERSISTENT);
    {{- end}}
    {{- end}}
    {{- end}}
    {{- end}}
    return SUCCESS;
}

zend_module_entry {{.BaseName}}_module_entry = {STANDARD_MODULE_HEADER,
                                         "{{.BaseName}}",
                                         {{if .Functions}}ext_functions{{else}}NULL{{end}},             /* Functions */
                                         PHP_MINIT({{.BaseName}}),  /* MINIT */
                                         NULL,                      /* MSHUTDOWN */
                                         NULL,                      /* RINIT */
                                         NULL,                      /* RSHUTDOWN */
                                         NULL,                      /* MINFO */
                                         "1.0.0",                   /* Version */
                                         STANDARD_MODULE_PROPERTIES};
