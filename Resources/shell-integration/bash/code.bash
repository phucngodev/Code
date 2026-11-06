# Code cannot load this into bash by itself. To have agents started through
# Code's shims, end ~/.bashrc or ~/.bash_profile with
#
#   [ -n "$CODE_CDE" ] && source "${CODE_CDE%/*}/../Resources/shell-integration/bash/code.bash"
#
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
