# Code adds this directory to XDG_DATA_DIRS only so that fish finds this
# file; programs started from the shell should see the user's value.
set --local code_data_dir (string replace --regex -- '/fish/vendor_conf.d$' '' (status dirname))
set --local data_dirs (string split -- : "$XDG_DATA_DIRS")
if set --local index (contains --index -- $code_data_dir $data_dirs)
    set --erase data_dirs[$index]
    if set --query data_dirs[1]
        set --global --export XDG_DATA_DIRS (string join -- : $data_dirs)
    else
        set --erase --global XDG_DATA_DIRS
    end
end

# The first prompt comes after the user's config has set PATH, so the shims
# go in front of wherever the agents were installed.
if status is-interactive; and set --query CODE_SURFACE_ID CODE_SHIMS; and not set --query CODE_AGENTS_DISABLED
    function __code_agents_init --on-event fish_prompt
        functions --erase __code_agents_init
        set --local dir $CODE_SHIMS/$CODE_SURFACE_ID
        "$CODE_CDE" shims $dir; and set --global --export --prepend PATH $dir
    end
end
