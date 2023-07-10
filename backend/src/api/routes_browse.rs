use std::fs::read_dir;
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};

use axum::{Json, Router};
use axum::extract::Query;
use axum::routing::get;
use serde::{Deserialize, Serialize};

use crate::Error;

#[derive(Deserialize)]
pub struct Browse {
    path: String,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct File {
    name: String,
    size: u64,
    isdir: bool,
    modified: u64,
}

pub fn list_directory_name(base_dir: String, path: String) -> Result<Vec<File>, std::io::Error> {
    let fixed_string = format!("{base_dir}/{path}");
    return read_dir(fixed_string)?
        .filter(|dir_entry| dir_entry.is_ok())
        .map(|dir_entry| {
            let file = dir_entry?;
            return Ok(File {
                name: file.file_name().into_string().unwrap(),
                size: file.metadata()?.len(),
                isdir: file.metadata()?.is_dir(),
                modified: file
                    .metadata()?
                    .modified()?
                    .duration_since(UNIX_EPOCH)
                    .unwrap()
                    .as_secs(),
            });
        })
        .collect();
}

pub async fn browse_response(lock: Arc<RwLock<SystemTime>>, base_dir: String, path: Option<Query<Browse>>) -> Result<Json<Vec<File>>, Error> {
    let p: String = match path {
        None => "".into(),
        Some(x) => x.path.to_owned(),
    };
    let mut m_lock = lock.write().unwrap();
    *m_lock = SystemTime::now();
    let json_result = list_directory_name(base_dir, p);
    return match json_result {
        Ok(json) => Ok(Json(json)),
        Err(_) => Err(Error::Fail)
    };
}

pub fn routes(lock: Arc<RwLock<SystemTime>>, base_dir: String) -> Router {
    Router::new()
        .route("/api/browse", get(move |x: _| {
            let lock_cloned = Arc::clone(&lock);
            browse_response(lock_cloned, base_dir, x)
        }))
}