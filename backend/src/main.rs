use std::fs::File;
use std::net::SocketAddr;
use std::path::PathBuf;
use std::sync::{Arc, RwLock};
use std::time::SystemTime;

use axum::Router;
use axum_server::tls_rustls::RustlsConfig;
use log;
use simplelog::*;
use tower_http::services::ServeDir;

pub use self::error::{Error, Result};

mod api;
mod args;
mod error;

#[tokio::main]
async fn main() -> color_eyre::Result<()> {
    color_eyre::install()?;
    let lock: Arc<RwLock<SystemTime>> = Arc::new(RwLock::new(SystemTime::now()));
    let c_lock = Arc::clone(&lock);
    WriteLogger::init(
        LevelFilter::Info,
        Config::default(),
        File::create("/tmp/decky_fileserver.log").unwrap(),
    )
    .unwrap();

    let server = tokio::spawn(async move {
        let config = match RustlsConfig::from_pem_file(
            PathBuf::from(args::get_plugin_dir())
                .join("certs")
                .join("deckyfileserver_cert.pem"),
            PathBuf::from(args::get_plugin_dir())
                .join("certs")
                .join("deckyfileserver_key.pem"),
        )
        .await
        {
            Ok(x) => x,
            Err(err) => {
                log::error!("Failed to find TLS certificates: {}", err);
                return;
            }
        };

        let app = Router::new()
            .merge(api::routes_browse::routes(lock, args::get_base_dir()))
            .merge(api::routes_root::routes(args::get_plugin_dir()))
            .nest_service("/api/download", ServeDir::new(args::get_base_dir()));

        let addr = SocketAddr::from(([0, 0, 0, 0], args::get_port()));
        match axum_server::bind_rustls(addr, config)
            .serve(app.into_make_service())
            .await
        {
            Ok(_) => {}
            Err(err) => {
                log::error!("Error starting up the web server: {}", err);
            }
        };
    });

    log::info!(
        "Server started on port {}, sharing directory: {}",
        args::get_port(),
        args::get_base_dir()
    );

    while !server.is_finished()
        && SystemTime::now()
            .duration_since(*c_lock.read().unwrap())
            .unwrap()
            .as_secs()
            < 60
    {
        tokio::time::sleep(tokio::time::Duration::from_secs(1)).await;
    }
    log::info!("Shutting down server.");
    server.abort();
    return Ok(());
}
