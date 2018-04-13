load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "ndb.go",
        "read.go",
        "write.go",
    ],
    importpath = "aqwari.net/encoding/ndb",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "read_test.go",
        "write_test.go",
    ],
    embed = [":go_default_library"],
)
