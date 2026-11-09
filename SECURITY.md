# Security

raster-go processes buffers decoded from untrusted files. Dimensions come from
hostile input, so every entry point validates its planes (width, height, stride,
buffer length, integer overflow) and its Limits before any assembly runs or any
memory is allocated. Assembly is unexported and reachable only through those
checks; tests run it against guard pages.

Report a vulnerability privately through GitHub's "Report a vulnerability" on
this repository's Security tab. Please do not open a public issue for one.
Include a failing input (synthetic, never real documents) if you have it.
