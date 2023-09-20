# Git hooks

This repository contains local Git hooks that are intended to increase
consistency in the naming of branches and tags, and the formatting of commit
messages.

The hooks are in the [`.githooks`](../.githooks) directory. Configure Git to
use these hooks:

```shell
make githooks
```

You only need to execute this command once for each clone of the repository.
