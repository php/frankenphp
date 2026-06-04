use futures::StreamExt;
use std::env::args;
use wtr_watcher::Watch;

async fn show(e: wtr_watcher::Event) {
    println!("{}", serde_json::to_string(&e).unwrap())
}

#[tokio::main(flavor = "current_thread")]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let p = args().nth(1).unwrap_or_else(|| ".".to_string());
    tokio::select! {
        _ = Watch::try_new(&p)?.for_each(show) => Ok(()),
        _ = tokio::signal::ctrl_c() => Ok(()),
    }
}
