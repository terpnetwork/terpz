use std::env;
use std::io::{self, Read, Write};

fn parse_pair(s: &str) -> (lean_stwo_dummy::M31, lean_stwo_dummy::M31) {
    let (a, b) = s.split_once(',').expect("a,b");
    (
        lean_stwo_dummy::M31::new(a.parse().unwrap()).unwrap(),
        lean_stwo_dummy::M31::new(b.parse().unwrap()).unwrap(),
    )
}

fn main() {
    let mut args = env::args().skip(1);
    let cmd = args.next().expect("prove|verify|reject-dummy");
    match cmd.as_str() {
        "prove" => {
            let pairs: Vec<_> = args.map(|s| parse_pair(&s)).collect();
            let proof = lean_stwo_dummy::prove_fold(&pairs).expect("prove");
            io::stdout().write_all(&proof).unwrap();
        }
        "verify" => {
            let pairs: Vec<_> = args.map(|s| parse_pair(&s)).collect();
            let mut proof = Vec::new();
            io::stdin().read_to_end(&mut proof).unwrap();
            lean_stwo_dummy::verify_fold(&proof, &pairs).expect("verify");
        }
        "reject-dummy" => {
            let mut blob = Vec::new();
            io::stdin().read_to_end(&mut blob).unwrap();
            lean_stwo_dummy::reject_dummy_n(&blob).expect("should not succeed");
        }
        other => panic!("unknown {other}"),
    }
}
