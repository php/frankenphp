# Early Hints (Pistas Tempranas)

FrankenPHP soporta nativamente el [código de estado 103 Early Hints](https://developer.chrome.com/blog/early-hints/).
El uso de Early Hints puede mejorar el tiempo de carga de sus páginas web hasta en un 30%.

```php
<?php

header('Link: </style.css>; rel=preload; as=style');
headers_send(103);

// sus algoritmos lentos y consultas SQL 🤪

echo <<<'HTML'
<!DOCTYPE html>
<title>Hola FrankenPHP</title>
<link rel="stylesheet" href="style.css">
HTML;
```

Early Hints están soportados tanto en el modo normal como en el modo [worker](worker.md).
