use futures::StreamExt;
use wtr_watcher::Watch;

#[tokio::main(flavor = "current_thread")]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let show = |e| async move { println!("{e:?}") };
    let events = Watch::try_new(".")?;
    events.for_each(show).await;
    Ok(())
}
