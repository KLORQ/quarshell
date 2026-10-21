// SPDX-License-Identifier: Apache-2.0

//! Pinned-upstream policy oracle for the differential conformance corpus.
//!
//! Without flags this exposes the upstream `openshell-policy` authored-policy
//! parser. With `--validate` it also runs the full runtime policy validator.
//! It is test material; it is not part of the Go product or release artifacts.

use std::path::Path;

use openshell_policy::{parse_sandbox_policy, validate_sandbox_policy};

fn main() {
    let mut validate = false;
    let mut path = None;
    for argument in std::env::args().skip(1) {
        if argument == "--validate" {
            validate = true;
        } else {
            path = Some(argument);
        }
    }
    let Some(path) = path else {
        eprintln!("usage: policy-oracle [--validate] <file>");
        std::process::exit(2);
    };
    let source = match std::fs::read_to_string(Path::new(&path)) {
        Ok(source) => source,
        Err(_) => {
            println!("{{\"accepted\":false,\"error_code\":\"io\"}}");
            return;
        }
    };
    match parse_sandbox_policy(&source) {
        Ok(policy) => {
            if validate && validate_sandbox_policy(&policy).is_err() {
                println!("{{\"accepted\":false,\"error_code\":\"invalid\"}}");
                return;
            }
            println!("{{\"accepted\":true,\"version\":{}}}", policy.version);
        }
        Err(_) => println!("{{\"accepted\":false,\"error_code\":\"invalid\"}}"),
    }
}
