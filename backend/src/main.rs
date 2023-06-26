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

fn get_base_dir() -> String {
    return std::env::args()
        .nth(1)
        .expect("Missing first argument: base_dir");
}

fn get_port() -> u16 {
    return match std::env::args().nth(2) {
        Some(val) => match val.parse() {
            Ok(parsed) => parsed,
            Err(e) => {
                println!("{}", e);
                println!("Failed to parse Port, probably not a valid number. Setting port to 9999");
                9999
            }
        },
        None => {
            println!("No val found, setting port to 9999");
            9999
        }
    };
}

fn get_plugin_dir() -> String {
    return std::env::args()
        .nth(3)
        .expect("Missing third argument: plugin_dir");
}

#[tokio::main]
async fn main() {
    let lock: Arc<RwLock<SystemTime>> = Arc::new(RwLock::new(SystemTime::now()));
    let c_lock = Arc::clone(&lock);

    let server = tokio::spawn(async move {
        let config = RustlsConfig::from_pem_file(
            PathBuf::from(get_plugin_dir()).join("certs").join("deckyfileserver_cert.pem"),
            PathBuf::from(get_plugin_dir()).join("certs").join("deckyfileserver_key.pem"),
        ).await.unwrap();

        let app = Router::new()
            .merge(api::routes_browse::routes(lock, get_base_dir()))
            .merge(api::routes_root::routes(get_plugin_dir()))
            .nest_service("/api/download", ServeDir::new(get_base_dir()));

        let addr = SocketAddr::from(([0, 0, 0, 0], get_port()));
        axum_server::bind_rustls(addr, config)
            .serve(app.into_make_service())
            .await
            .unwrap();
    });

    while SystemTime::now()
        .duration_since(*c_lock.read().unwrap())
        .unwrap()
        .as_secs()
        < 60
    {
        tokio::time::sleep(tokio::time::Duration::from_secs(1)).await;
    }
    server.abort();
}
