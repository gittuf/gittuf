// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

//! SKETCH (cedar-integration-sketch): does a gittuf rule file change weaken
//! protection of refs/heads/main? Usage: rulefile_diff <compiled-before> <compiled-after>
//! Checks, on refs/heads/main only, that every request allowed by AFTER is
//! allowed by BEFORE (cedar-policy-symcc + cvc5). Exit 0 if it holds, 1 if a
//! counterexample (a weakening) exists, 2 on error. Counterexamples are
//! replayed through Cedar's concrete authorizer.

use std::{env, fs, path::Path, process::ExitCode, str::FromStr};

use cedar_policy::{Authorizer, Decision, PolicySet, Schema};
use cedar_policy_symcc::{solver::LocalSolver, CedarSymCompiler, CompiledPolicySet};

const SCOPE_MAIN: &str =
    "forbid(principal, action, resource) unless { resource.name == \"refs/heads/main\" };\n";

fn load(dir: &Path) -> (String, String) {
    let p = fs::read_to_string(dir.join("policies.cedar")).expect("policies.cedar");
    let s = fs::read_to_string(dir.join("schema.cedarschema")).expect("schema.cedarschema");
    (p, s)
}

#[tokio::main(flavor = "current_thread")]
async fn main() -> ExitCode {
    let args: Vec<String> = env::args().collect();
    if args.len() != 3 {
        eprintln!("usage: rulefile_diff <compiled-before> <compiled-after>");
        return ExitCode::from(2);
    }
    let (before, schema_before) = load(Path::new(&args[1]));
    let (after, schema_after) = load(Path::new(&args[2]));
    // The POC schema depends on the principals; both sides must agree.
    if schema_before != schema_after {
        eprintln!("before and after compile to different schemas; add the same principals to both");
        return ExitCode::from(2);
    }
    let schema = Schema::from_cedarschema_str(&schema_after).expect("schema").0;
    let ps_before = PolicySet::from_str(&(before + SCOPE_MAIN)).expect("before");
    let ps_after = PolicySet::from_str(&(after + SCOPE_MAIN)).expect("after");

    let mut compiler = CedarSymCompiler::new(LocalSolver::cvc5().expect("cvc5 (set CVC5=path)")).expect("solver");
    let mut weakened = false;
    for req_env in schema.request_envs() {
        let c_after = CompiledPolicySet::compile(&ps_after, &req_env, &schema).expect("compile after");
        let c_before = CompiledPolicySet::compile(&ps_before, &req_env, &schema).expect("compile before");
        match compiler.check_implies_with_counterexample_opt(&c_after, &c_before).await.expect("solver") {
            None => println!("unsat ({} / {} / {}): on refs/heads/main, AFTER allows only what BEFORE allows",
                req_env.principal(), req_env.action(), req_env.resource()),
            Some(cex) => {
                weakened = true;
                let a = Authorizer::new();
                let d_before = a.is_authorized(&cex.request, &ps_before, &cex.entities).decision();
                let d_after = a.is_authorized(&cex.request, &ps_after, &cex.entities).decision();
                println!("COUNTEREXAMPLE: the change weakens refs/heads/main\n{cex}");
                println!("replayed in Cedar: before={d_before:?} after={d_after:?} (expected before=Deny after=Allow: {})",
                    d_before == Decision::Deny && d_after == Decision::Allow);
            }
        }
    }
    if weakened { ExitCode::from(1) } else { ExitCode::SUCCESS }
}
