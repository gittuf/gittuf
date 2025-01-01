# Setting Constraints on gittuf Rules and Rule Files

Last Modified: December 16, 2024

Status: Draft

Currently, gittuf does not support setting minimum constraints on rules or rule
files. For example, it is not possible to set constraints such as "all rules
that apply to the main branch must have a minimum threshold of 2". Similarly, we
cannot set constraints such as "all rule files must have a threshold of 2".
Instead, maintaining a baseline is left to the author of each individual rule in
the policy.

## Constraints Model

Constraints can be thought of as rules that apply to typical gittuf rules or
rule files (which are delegated metadata from a rule). Therefore, constraints
cannot be declared alongside other rules. The responsibility for declaring
constraints is left to the root of trust metadata, i.e., the repository's
owners.

## Rule Constraints

A rule constraint applies to any rule that applies to some repository namespace
that the constraint is also for. The following table describes shows examples
for whether or not a constraint applies to a rule based on their respective
namespace patterns.

| Rule Constraint Pattern | Rule Pattern           | Applies |
| ----------------------- | ---------------------- | ------- |
| `refs/heads/main`       | `refs/heads/main`      | Yes     |
| `refs/heads/main`       | `refs/heads/feature/*` | No      |
| `refs/tags/v*`          | `refs/tags/v*`         | Yes     |
| `refs/tags/v*`          | `refs/tags/v1*`        | Yes     |
| `refs/tags/*`           | `refs/tags/v*`         | Yes     |
| `refs/tags/v*.0.0`      | `refs/tags/v*`         | Yes     |

Notice that the pattern employed by a constraint may be more specific than those
employed by rules.

## Rule File Constraints

A rule file constraint applies to all rule files in the repository. As with rule
constraints, it's primarily a threshold constraint. A single rule file
constraint can be set that applies to all rule files in the repository ensuring
each one's threshold meets the constraint's minimum. For example, a minimum of
`2` ensures that all rule files have at least two signatures. Significantly,
this does not require all rules to have a threshold of `2` or higher. A rule
that has a threshold of `1` cannot be turned into a delegation (by creating a
rule file) until the threshold is updated.

## Open Questions

1. All constraints considered so far are really threshold constraints. How
   parametrizable should constraints be?
