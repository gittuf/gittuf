# Comparison: gittuf vs Guix vs sequoia-git

This document compares gittuf with other systems that integrate trust, signing, and verification into Git-based workflows.  
The criteria below are directly based on [gittuf’s stated goals](https://gittuf.dev/goals.html).

| Goal                                           | gittuf | Guix | sequoia-git |
|-----------------------------------------------|--------|------|--------------|
| Enforce trust for all Git refs                | ✅     | ❌   | ✅           |
| Store trust metadata in Git                   | ✅     | ✅   | ✅           |
| Make trust decisions based on policy          | ✅     | ❌   | ❌           |
| Support decentralized and delegated trust     | ✅     | ❌   | ❌           |

---

**Legend:**

- ✅ = Fully supported  
- ❌ = Not supported  

> **Note:** These are the core goals of gittuf. RSL (Reference State Log) and other internal features support these foundational principles.
