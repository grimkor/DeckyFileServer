use std::fs::read_dir;
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};

use axum::extract::Query;
use axum::routing::get;
use axum::{Json, Router};
use color_eyre::eyre::eyre;
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

pub fn list_directory_name(base_dir: String, path: String) -> color_eyre::Result<Vec<File>> {
    let fixed_string = format!("{base_dir}/{path}");
    return read_dir(fixed_string)?
        .map(|dir_entry| -> color_eyre::Result<File> {
            let file = dir_entry?;
            let name: String = match file.file_name().to_str() {
                None => return Err(eyre!("Unable to convert filename: {:?}", file.file_name())),
                Some(name) => name.into(),
            };
            return Ok(File {
                name,
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
        .inspect(|res| match res {
            Ok(_) => {}
            Err(err) => {
                log::error!("Error getting file properties: {}", err)
            }
        })
        .filter(|res| res.is_ok())
        .collect();
}

pub async fn browse_response(
    lock: Arc<RwLock<SystemTime>>,
    base_dir: String,
    path: Option<Query<Browse>>,
) -> Result<Json<Vec<File>>, Error> {
    let p: String = match path {
        None => "".into(),
        Some(x) => x.path.to_owned(),
    };
    let mut m_lock = lock.write().unwrap();
    *m_lock = SystemTime::now();
    let json_result = list_directory_name(base_dir, p);
    return match json_result {
        Ok(json) => Ok(Json(json)),
        Err(err) => {
            log::error!("{}", err);
            Err(Error::Fail)
        }
    };
}

pub fn routes(lock: Arc<RwLock<SystemTime>>, base_dir: String) -> Router {
    Router::new().route(
        "/api/browse",
        get(move |x: _| {
            let lock_cloned = Arc::clone(&lock);
            browse_response(lock_cloned, base_dir, x)
        }),
    )
}
