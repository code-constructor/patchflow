import "/assets/vendor/turbo.js"
import { Application } from "@hotwired/stimulus"
import DiffViewerController from "/assets/controllers/diff_viewer_controller.js"
import ReviewDocumentController from "/assets/controllers/review_document_controller.js"

const application = Application.start()
application.debug = false
application.register("diff-viewer", DiffViewerController)
application.register("review-document", ReviewDocumentController)
