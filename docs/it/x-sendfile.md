# Servire in modo efficiente file statici di grandi dimensioni (`X-Sendfile`/`X-Accel-Redirect`)

Di solito, i file statici possono essere serviti direttamente dal server web,
ma a volte è necessario eseguire del codice PHP prima di inviarli:
controllo degli accessi, statistiche, intestazioni HTTP personalizzate...

Sfortunatamente, l'utilizzo di PHP per gestire file statici di grandi dimensioni è inefficiente rispetto a
utilizzo diretto del web server (sovraccarico di memoria, prestazioni ridotte...).

FrankenPHP consente di delegare l'invio di file statici al server web
**dopo** l'esecuzione del codice PHP personalizzato.

Per poterlo fare, l'applicazione PHP deve semplicemente definire un'intestazione HTTP personalizzata
contenente il percorso del file da servire. FrankenPHP si occuperà del resto.

Questa funzionalità è nota come **`X-Sendfile`** per Apache e **`X-Accel-Redirect`** per NGINX.

Negli esempi seguenti, assumiamo che la document root del progetto sia la cartella `public/`
e che vogliamo utilizzare PHP per servire i file archiviati all'esterno di `public/`,
da una cartella denominata `private-files/`.

## Configurazione di X-Accel-Redirect in Caddyfile

Innanzitutto, aggiungere la seguente configurazione a `Caddyfile` per abilitare questa funzione:

```patch
	root public/
	# ...

+	# Necessario per Symfony, Laravel e altri progetti che usano il componente HttpFoundation
+	request_header X-Sendfile-Type x-accel-redirect
+	request_header X-Accel-Mapping ../private-files=/private-files
+
+	intercept {
+		@accel header X-Accel-Redirect *
+		handle_response @accel {
+			root private-files/
+			rewrite * {resp.header.X-Accel-Redirect}
+			method * GET
+
+			# Rimuove l'header X-Accel-Redirect impostato da PHP, per maggior sicurezza
+			header -X-Accel-Redirect
+
+			file_server
+		}
+	}

	php_server
```

## PHP puro

Impostare il percorso relativo del file (da `private-files/`) come valore dell'header `X-Accel-Redirect`:

```php
header('X-Accel-Redirect: file.txt');
```

## Progetti che utilizzano il componente Symfony HttpFoundation (Symfony, Laravel, Drupal...)

Vedere la [documentazione di Symfony](symfony.md#servire-grandi-file-statici-x-sendfile) per i dettagli sull'uso di questa funzionalità con Symfony HttpFoundation.
