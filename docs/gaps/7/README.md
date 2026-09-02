# Application-Defined Custom Fields

## Metadata

* **Number:** 7
* **Title:** Application-Defined Custom Fields
* **Implemented:** Yes
* **Withdrawn/Rejected:** No
* **Sponsors:** Paulo Gomes (pjbgf)
* **Related GAPs:** [GAP-2](/docs/gaps/2/README.md), [GAP-3](/docs/gaps/3/README.md)
* **Last Modified:** August 27, 2026

## Abstract

gittuf records every reference update as a signed entry in the Reference State
Log (RSL). Applications that build on gittuf often have context about a change
that the standard entry does not capture, such as the actor who pushed it or
the server that processed it. This GAP introduces custom fields,
application-defined key-value pairs carried inside an RSL entry's commit
message. Custom fields are opaque to gittuf and are never consulted during
verification.

## Motivation

gittuf's RSL is increasingly used as a building block by other software rather
than only through the gittuf CLI. A forge, a mirroring service, or an internal
platform tool may want to attach its own context to the record of a reference
update. Until now the entry schema was closed: the only way to associate extra
data with an entry was to model it as a separate gittuf construct (e.g., a
separate annotation entry or an attestation).

The concrete driver is applications that want the RSL to double as an audit
ledger for a repository. Each push already produces a signed, ordered, and
tamper-evident record. If that record can also carry a small amount of
application context, the application gains a durable audit trail that is
co-located with the change it describes and covered by the same signature.

## Specification

### Field Structure

An RSL entry may carry zero or more custom fields after its standard fields.
For an annotation entry, they appear before the message block. A field is a
single line of the form `custom.<namespace>/<name>: <value>`.

```
RSL Reference Entry

ref: <ref name>
targetID: <target ID>
number: <number>
custom.<namespace>/<name>: <value>
```

Field names follow the Kubernetes annotation convention: a lowercase DNS
subdomain the application controls (for example `gitforge.com`), a `/`, and a
name of at most 63 lowercase characters that starts and ends alphanumeric,
with `.`, `_`, or `-` permitted in between. Namespacing a key to a domain the
application controls avoids collisions between applications writing to the
same RSL.

The whole key, including the `custom.` prefix, MUST be fewer than 250
characters and MUST be unique within a canonical entry. Values MAY contain
ASCII letters, digits, and the characters `-+./,()_@:=%#?&~` as well as
internal spaces. This covers common value shapes such as emails, URLs,
RFC 3339 timestamps, and base64. Values MUST NOT have leading or trailing
spaces and MUST be fewer than 500 characters. An entry MUST NOT carry more
than 20 custom fields.

A recorded reference entry carrying one field looks like:

```
RSL Reference Entry

ref: refs/heads/main
targetID: a1b2c3d4e5f60718293a4b5c6d7e8f9012345678
number: 42
custom.gitforge.com/pusher: jane (01ARZ3NDEKTSV4RRFFQ69G5FAV)
```

Parentheses and internal spaces are in the value alphabet, so pairing a handle
with the forge's account identifier needs no encoding.

Values SHOULD be human-readable, so that an operator inspecting an entry with
standard Git tooling can read them directly without decoding. Where content
needs characters the value alphabet does not permit, such as line breaks, the
application SHOULD base64 the value rather than expect the grammar to widen.

Custom fields are not intended to carry large amounts of data. An application
with such a payload is better off storing it elsewhere and recording only a
pointer to it here, for example a URL to an external service or an internal
Git ref that points to a blob.

### Supported Types

All three RSL entry types carry custom fields: reference entries, annotation
entries, and propagation entries. The key grammar, value alphabet, and length
and count limits are identical across them and are enforced by a single shared
implementation.

Nothing outside the RSL carries custom fields, including policy metadata and
the commits that record it. The RSL is where an application's context about a
change belongs, because the record of the change and the context describing it
then live in one signed object. Policy states describe who is trusted to do
what, and a policy change is already recorded by an RSL entry, so an
application that wants to mark one has somewhere to put the mark without
widening the documents verification reads.

### Reading and Writing

These constraints are enforced on write. Writers drop fields with empty values
and sort the rest by name so the encoding is deterministic and the resulting
commit is stable across implementations. An empty value therefore means the
field is absent rather than present and blank.

Readers require custom fields to appear after the standard fields and reject
entries that interleave them, but accept the custom fields themselves in any
order. When a non-canonical entry repeats a name, the reader uses the first
value it encounters. A field whose key or value falls outside the grammar is
ignored, as is a field with an empty value, and the total count is capped. The
rule is the same on both sides, so a field a canonical writer could not have
produced is never surfaced by a read. This matters because consumers, and
`gittuf rsl log`, render values as recorded, and an entry is a commit message
into which anything can be written directly with Git.

Ignoring such a field rather than rejecting the entry keeps a single bad line
from making an entry unreadable, which would turn a cosmetic defect into a
traversal failure for a log gittuf still has to walk.

### Relationship to Verification

Custom fields MUST NOT influence verification. gittuf treats them as opaque
metadata whose presence, absence, or contents never change a verification
outcome. The entry's signature covers the whole commit message, including the
custom fields, so they cannot be altered without invalidating the entry. This
is the only property gittuf provides for them.

### Tooling

The gittuf CLI does not create custom fields. They are aimed at applications
that embed gittuf as a library and populate the fields programmatically.
`gittuf rsl log` does render the fields an entry carries, so an operator can
read them without knowing which names to ask for:

```
entry a5ea2c6ee7e8b577f6be6fcee5b06e6cac7166fa

  Ref:    refs/heads/main
  Target: 6cb8e5c546eab3d0e1d245014de8003febb8e9b3
  Number: 42
  Custom Fields:
    custom.gitforge.com/pusher: jane (01ARZ3NDEKTSV4RRFFQ69G5FAV)
```

The fields remain readable with standard Git tooling as well, since they are
plain lines in a commit message:

```
git cat-file -p <rsl-entry-oid>
```

## Reasoning

### Why not attestations

Attestations (see [GAP-3](/docs/gaps/3/README.md)) are the right tool when the
extra data is itself security-relevant and needs an independent, verifiable
structure: a DSSE envelope with a predicate type, stored under a path in
`refs/gittuf/attestations`. Each attestation is a blob under a path, so
recording one writes the blob, the trees along its path, and a commit on the
attestations ref. Because the attestations ref itself moves, that movement in
turn requires its own RSL entry. A single enriched change can therefore add on
the order of five objects to the repository. Custom fields are text inside a
commit message that already exists, so recording a reference update with them
writes exactly the same single entry commit that gittuf would write anyway.
For advisory context that does not need to be independently verifiable, the
attestation cost per change is disproportionate.

### Why not annotations

Annotations are entries in their own right. Using one to attach context to a
change means writing a second commit on the RSL ref that points back at the
original entry. That roughly doubles the object cost of recording a change and
splits the context away from the entry it describes, so a reader has to
correlate two objects to reconstruct one event.

### Cost of storing data outside the RSL Entry

An RSL reference entry is a single commit whose tree is the empty tree. The
empty tree object is written once and shared by every entry thereafter, so
each recorded change contributes one new object to the repository. Custom
fields change the bytes of that commit but not the number of objects.

Measured against a year of real reference activity, in Git objects. The
entry-only row counts the reference updates GitHub records for `go-git` and
`kubernetes` over a twelve-month window, one RSL entry per update. The other
two rows apply the per-change object multipliers described above to that
count, so only the first row is measured:

| Recording strategy | go-git | kubernetes |
| ------------------ | -----: | ---------: |
| Entry only         |    924 |      3,581 |
| + annotation (2x)  |  1,848 |      7,162 |
| + attestation (5x) |  4,620 |     17,905 |

Higher-traffic repositories see larger numbers, but the cost stays at one
object per change, so an entry-only RSL remains cheap to store, walk, and
pack. The multipliers understate the difference, because the added objects are
heavier: distinct blobs and trees delta far less cleanly than near-identical
RSL commits do. Object count is what connectivity checks, reachability walks,
and bitmap builds pay for, so keeping enrichment inside the entry keeps those
operations cheap.

### Not providing native support for custom fields

gittuf could carry no fields at all and leave applications to craft the commit
objects themselves, writing an RSL entry with whatever message they want.
Nothing in the object format prevents it.

That trades a small extension point for a much larger one. An application
writing its own objects needs the primitives to build a commit, sign it, move
the reference, and record the update, so gittuf would have to publish those as
API and keep them stable across releases. A field grammar and a validator are
the narrower contract: an extension point that is specified, versioned with
the entry format, and stable enough to build on, while the objects stay
gittuf's to write.

It also keeps the log interoperable. Without a grammar each application
invents its own key and value syntax, so an entry is readable only by whoever
wrote it, and a hand-rolled line could produce an entry gittuf itself refuses
to parse.

## Backwards Compatibility

This GAP does not impact the backwards compatibility of gittuf. Entries
without custom fields are unchanged, byte for byte, and the standard fields
keep their existing order and meaning. The entry grammar is otherwise
untouched: a body a reader accepted before this GAP is still accepted, and one
it rejected is still rejected. Custom fields are additive lines appended after
the standard fields. Readers that predate this GAP ignore unknown lines in an
entry body, so they tolerate custom fields without modification, though they
will not surface them. Because custom fields are excluded from verification,
an older client and a newer client reach the same verification decision for
the same entry.

Policy metadata schemas are not modified by this GAP.

## Security

Custom fields MUST NOT influence verification. Because gittuf never reads them
when deciding whether a change is authorized, a malicious or malformed field
cannot change a verification outcome. Applications MUST NOT make security
decisions based on custom fields.

The fields are tamper-evident only to the degree the enclosing entry is. The
signature over the entry's commit message covers them, so a recorded value
cannot be altered without detection, but this does not mean the value was true
when it was written. A forge that records `custom.gitforge.com/pusher` is
asserting that claim, and gittuf attests only that the claim has not changed
since the entry was signed. Data that needs to be independently verifiable
belongs in an attestation (see [GAP-3](/docs/gaps/3/README.md)), not a custom
field.

Consumers MUST treat custom fields as untrusted input and make no assumptions
about their contents unless they generated the data themselves, validated by the signature on the object. The intended
pattern is that the producer is the consumer: a forge reads back the fields
it stamped under its own namespace and treats everything else as opaque. A
field is asserted by whoever created and signed the entry that contains it,
and entries in the same log may be signed by different actors over time. The
namespace in a key identifies a writer by convention only: nothing prevents a
writer from using another application's namespace, so the key alone MUST NOT
be used to attribute a value. An application that controls the write path can
enforce the convention it relies on, for example a forge that rejects pushed
entries carrying fields under its own namespace, leaving only the fields it
stamped itself. None of this changes gittuf's stance: gittuf never consults
custom fields during verification.

The read path is bounded against hostile input. Keys and values are limited in
length, the number of fields per entry is capped, and oversized fields are
dropped rather than rejected during traversal, so a crafted entry cannot force
a reader to allocate unbounded memory or stall while walking the log. On the
write path the character sets and length limits are enforced, and values are
constrained to a single line so a field cannot inject additional entry lines
or forge a message block.

## Implementation

The key grammar, value alphabet, and limits are implemented once in a shared
validation package, which also owns the encoding, so every entry type emits
the same lines from the same field set. The value alphabet is exported, so an
application that maps its own input into a value, such as a forge sanitizing a
user handle, sanitizes against the same alphabet gittuf validates rather than
restating it.

In the RSL package, `ReferenceEntry`, `AnnotationEntry`, and
`PropagationEntry` each carry a `CustomFields` map. The keyed read lives on a
`CustomFieldEntry` interface rather than on `Entry`, so an existing
implementation of `Entry` keeps satisfying it and callers holding an `Entry`
reach the fields with a type assertion. The read is keyed: a caller asks for a
specific field by name and gets its value without copying the map, so
per-entry reads during log traversal do not allocate. There is deliberately no
accessor that enumerates fields. An application knows the names of the fields
it wrote, and a keyed interface steers consumers toward reading their own
fields rather than data they cannot vouch for. Validation, encoding, and
parsing follow the Specification. Fields are supplied when an entry is built,
so an invalid field set fails while the commit message is assembled and before
any object is written.

The gittuf CLI exposes no command to set custom fields. They are populated by
applications that use gittuf as a library. Rendering the log is the one place
gittuf enumerates fields, so `gittuf rsl log` reads the map on the concrete
entry type rather than through the interface.

## References

* [gittuf Design Document](/docs/design-document.md)
* [GAP-2: gittuf on the Forge](/docs/gaps/2/README.md)
* [GAP-3: Authentication Evidence Attestations](/docs/gaps/3/README.md)
