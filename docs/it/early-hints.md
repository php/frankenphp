# Early Hints

FrankenPHP supporta nativamente il [codice di stato 103 Early Hints](https://developer.chrome.com/blog/early-hints/).
Utilizzando Early Hints si può migliorare il tempo di caricamento delle pagine web del 30%.

```php
<?php

header('Link: </style.css>; rel=preload; as=style');
headers_send(103);

// qualcosa di lento... 🤪

echo <<<'HTML'
<!DOCTYPE html>
<title>Hello FrankenPHP</title>
<link rel="stylesheet" href="style.css">
HTML;
```

Gli Early Hints sono supportati sia dalla modalità normale sia da quella [worker](worker.md).
