use std::{
    sync::{Arc, RwLock},
};
use std::net::SocketAddr;
use std::path::PathBuf;
use std::time::SystemTime;

use axum::Router;
use axum_server::tls_rustls::RustlsConfig;
use tower_http::services::ServeDir;

pub use self::error::{Error, Result};

mod api;
mod error;
mod args;

#[tokio::main]
async fn main() {
    let lock: Arc<RwLock<SystemTime>> = Arc::new(RwLock::new(SystemTime::now()));
    let c_lock = Arc::clone(&lock);

    let server = tokio::spawn(async move {
        let config = match RustlsConfig::from_pem_file(
            PathBuf::from(args::get_plugin_dir()).join("certs").join("deckyfileserver_cert.pem"),
            PathBuf::from(args::get_plugin_dir()).join("certs").join("deckyfileserver_key.pem"),
        ).await {
            Ok(x) => x,
            Err(err) => {
                eprintln!("Failed to find TLS certificates: {}", err);
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
            .await {
            Ok(_) => {}
            Err(err) => {
                eprintln!("Error starting up the web server: {}", err);
            }
        };
    });

    while !server.is_finished() && SystemTime::now()
        .duration_since(*c_lock.read().unwrap())
        .unwrap()
        .as_secs()
        < 60
    {
        tokio::time::sleep(tokio::time::Duration::from_secs(1)).await;
    }
    server.abort();
}
