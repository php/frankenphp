# メトリクス

[Caddyのメトリクス](https://caddyserver.com/docs/metrics)が有効になっていると、FrankenPHPは以下のメトリクスを公開します：

- `frankenphp_total_threads`: PHPスレッドの総数
- `frankenphp_busy_threads`: 現在リクエストを処理中のPHPスレッド数。なお、実行中のワーカーは常にスレッドを消費します
- `frankenphp_queue_depth`: 通常のキューに入っているリクエストの数
- `frankenphp_total_workers{worker="[worker_name]"}`: ワーカーの総数
- `frankenphp_busy_workers{worker="[worker_name]"}`: 現在リクエストを処理中のワーカーの数
- `frankenphp_worker_request_time{worker="[worker_name]"}`: すべてのワーカーがリクエスト処理に費やした時間
- `frankenphp_worker_request_count{worker="[worker_name]"}`: すべてのワーカーが処理したリクエスト数
- `frankenphp_ready_workers{worker="[worker_name]"}`: 少なくとも一度は `frankenphp_handle_request` を呼び出したワーカーの数
- `frankenphp_worker_crashes{worker="[worker_name]"}`: ワーカーが予期せず終了した回数
- `frankenphp_worker_restarts{worker="[worker_name]"}`: ワーカーが意図的に再起動された回数
- `frankenphp_worker_queue_depth{worker="[worker_name]"}`: キューに入っているリクエストの数

ワーカーメトリクスの`[worker_name]`プレースホルダーは、Caddyfileに指定されたワーカー名に置き換えられます。ワーカー名が指定されていない場合は、ワーカーファイルの絶対パスが使用されます。
