load("@bazel_skylib//lib:paths.bzl", "paths")
load("@io_bazel_rules_go//go:def.bzl", "go_context")

def _go_sdk_tool_impl(ctx):
    # Locate the File object corresponding to the tool path. This is needed for
    # bazel to be able to create an executable symlink to that tool and to be
    # able to recreate the symlink if the tool changes.
    go = go_context(ctx)
    tool_path = paths.join(go.sdk_root.dirname, ctx.attr.goroot_relative_path)
    tool = None
    for f in go.sdk_tools:
        if f.path == tool_path:
            tool = f
            break
    if not tool:
        fail("could not locate SDK tool '%s'" % tool_path)

    # Declare an executable symlink to the tool File.
    out = ctx.actions.declare_file(ctx.attr.name)
    ctx.actions.symlink(output = out, target_file = tool, is_executable = True)
    return [DefaultInfo(files = depset([out]), executable = out)]

go_sdk_tool = rule(
    doc = "Declares a run target from the go SDK.",
    attrs = {
        "goroot_relative_path": attr.string(mandatory = True, doc = "Tool path relative to the go SDK root (GOROOT)"),
        "_go_context_data": attr.label(
            default = "@io_bazel_rules_go//:go_context_data",
        ),
    },
    implementation = _go_sdk_tool_impl,
    executable = True,
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)
