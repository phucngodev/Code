# Code cannot load this into bash by itself: bash reads BASH_ENV only for
# non-interactive shells. To have agents started through Code's shims in an
# interactive bash, end ~/.bashrc or ~/.bash_profile with
#
#   [ -n "$CODE_AGENTS_DIR" ] && source "$CODE_AGENTS_DIR/shell-integration/bash/code.bash"
#
# zsh and fish are wired up by Code's environment instead.
# Preserve a BASH_ENV the user already set for non-interactive shells.
if [[ -n "${CODE_BASH_ENV:-}" && -r "$CODE_BASH_ENV" ]]; then
    . "$CODE_BASH_ENV"
fi

# The first prompt comes after the user's config has set PATH, so the shims
# go in front of wherever the agents were installed.
if [[ $- == *i* && -n "$CODE_SURFACE_ID" && -n "$CODE_SHIMS" && -z "$CODE_AGENTS_DISABLED" ]]; then
    __code_agents_init() {
        PROMPT_COMMAND=${PROMPT_COMMAND/__code_agents_init;/}
        unset -f __code_agents_init
        local dir="$CODE_SHIMS/$CODE_SURFACE_ID"
        "$CODE_CDE" shims "$dir" && PATH="$dir:$PATH"
    }
    PROMPT_COMMAND="__code_agents_init;${PROMPT_COMMAND}"
fi
