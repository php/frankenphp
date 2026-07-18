# Usare la modalità classica

Senza alcuna configurazione aggiuntiva, FrankenPHP funziona in modalità classica. In questa modalità, FrankenPHP funziona come un tradizionale server PHP, servendo direttamente i file PHP. Ciò lo rende un sostituto perfetto per PHP-FPM o Apache con mod_php.

Similmente a Caddy, FrankenPHP accetta un numero illimitato di connessioni e utilizza un [numero fisso di thread](config.md#caddyfile-config) per servirle. Il numero di connessioni accettate e in coda è limitato solo dalle risorse di sistema disponibili.
Il pool di thread PHP funziona con un numero fisso di thread inizializzati all'avvio, paragonabile alla modalità statica di PHP-FPM. È anche possibile consentire ai thread di [ridimensionarsi automaticamente in fase di esecuzione](performance.md#max_threads), in modo simile alla modalità dinamica di PHP-FPM.

Le connessioni in coda attenderanno indefinitamente finché non sarà disponibile un thread PHP per servirle. Per evitarlo, si può utilizzare max_wait_time nella [configurazione](config.md#caddyfile-config) globale di FrankenPHP per limitare la durata di attesa di una richiesta per un thread PHP libero prima di essere rifiutata.
Inoltre, si può impostare un [timeout di scrittura](https://caddyserver.com/docs/caddyfile/options#timeouts) ragionevole.

Ogni istanza Caddy avvierà solo un pool di thread FrankenPHP, che sarà condiviso tra tutti i blocchi `php_server`.
