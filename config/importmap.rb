# Pin npm packages by running ./bin/importmap

pin "application"
pin "@hotwired/turbo-rails", to: "turbo.min.js"
pin "@hotwired/stimulus", to: "stimulus.min.js"
pin "@hotwired/stimulus-loading", to: "stimulus-loading.js"
pin_all_from "app/javascript/controllers", under: "controllers"
pin "dompurify" # @3.4.13
pin "marked" # @16.4.2
pin "diff2html", to: "diff2html--bundles--js--diff2html.min.js.js" # @3.4.56
