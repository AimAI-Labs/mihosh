" One Dark Pro palette by Binaryify, adapted for Vim's YAML syntax groups.
" https://github.com/Binaryify/OneDark-Pro/blob/master/src/themes/themeData.ts
set nomodeline noexrc noswapfile nobackup nowritebackup noundofile
set background=dark
if exists('+termguicolors') && $COLORTERM =~# 'truecolor\|24bit'
  set termguicolors
endif
filetype on
syntax on
let g:colors_name = 'onedark-pro'
highlight Normal     guifg=#abb2bf guibg=#282c34 ctermfg=145 ctermbg=235
highlight Comment    guifg=#5c6370 ctermfg=59
highlight Identifier guifg=#e06c75 ctermfg=168
highlight String     guifg=#98c379 ctermfg=114
highlight Number     guifg=#d19a66 ctermfg=173
highlight Constant   guifg=#d19a66 ctermfg=173
highlight Keyword    guifg=#c678dd ctermfg=176
highlight Type       guifg=#e5c07b ctermfg=180
highlight Label      guifg=#61afef ctermfg=75
highlight Special    guifg=#56b6c2 ctermfg=73
