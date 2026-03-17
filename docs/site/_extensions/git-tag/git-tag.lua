-- Shortcode that returns the latest git tag
return {
  ["git-tag"] = function(args, kwargs, meta)
    local handle = io.popen("git describe --tags --always")
    local result = handle:read("*a")
    handle:close()
    return pandoc.Str(result:gsub("%s+", ""))
  end
}
