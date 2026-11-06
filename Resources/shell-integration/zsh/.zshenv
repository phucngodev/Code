# Code points ZDOTDIR here so that zsh runs this first; the rest of startup
# must read the user's own files.
if [[ -n "${CODE_ZSH_ZDOTDIR+X}" ]]; then
    'builtin' 'export' ZDOTDIR="$CODE_ZSH_ZDOTDIR"
    'builtin' 'unset' 'CODE_ZSH_ZDOTDIR'
else
    'builtin' 'unset' 'ZDOTDIR'
fi

# Quoted builtins keep the user's aliases out of this file.
{
    'builtin' 'typeset' _code_file="${ZDOTDIR-$HOME}/.zshenv"
    [[ ! -r "$_code_file" ]] || 'builtin' 'source' '--' "$_code_file"
} always {
    'builtin' 'unset' '_code_file'
    # The first prompt comes after the user's config has set PATH, so the
    # shims go in front of wherever the agents were installed.
    if [[ -o 'interactive' && -n "$CODE_SURFACE_ID" && -n "$CODE_SHIMS" && -z "$CODE_AGENTS_DISABLED" ]]; then
        _code_agents_init() {
            precmd_functions=(${precmd_functions:#_code_agents_init})
            'builtin' 'unfunction' '_code_agents_init'
            'builtin' 'local' dir="$CODE_SHIMS/$CODE_SURFACE_ID"
            "$CODE_CDE" shims "$dir" && 'builtin' 'export' PATH="$dir:$PATH"
        }
        'builtin' 'typeset' -ga precmd_functions
        precmd_functions+=(_code_agents_init)
    fi
}
