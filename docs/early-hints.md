---
title: Sending HTTP 103 Early Hints from PHP with FrankenPHP
description: FrankenPHP natively supports the HTTP 103 Early Hints status code, letting PHP applications preload assets before the final response is ready.
---

# Early Hints

FrankenPHP natively supports the [103 Early Hints status code](https://developer.chrome.com/blog/early-hints/).
Using Early Hints can improve the load time of your web pages by 30%.

```php
<?php

header('Link: </style.css>; rel=preload; as=style');
headers_send(103);

// your slow algorithms and SQL queries 🤪

echo <<<'HTML'
<!DOCTYPE html>
<title>Hello FrankenPHP</title>
<link rel="stylesheet" href="style.css">
HTML;
```

Early Hints are supported both by the normal and the [worker](worker.md) modes.
