pub fn get_base_dir() -> String {
    return std::env::args()
        .nth(1)
        .expect("Missing first argument: base_dir");
}

pub fn get_port() -> u16 {
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

pub fn get_plugin_dir() -> String {
    return std::env::args()
        .nth(3)
        .expect("Missing third argument: plugin_dir");
}
