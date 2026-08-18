import "/assets/vendor/turbo.js"
import { Application } from "@hotwired/stimulus"
import DiffViewerController from "/assets/controllers/diff_viewer_controller.js"
import DiagramViewerController from "/assets/controllers/diagram_viewer_controller.js"
import ReviewDocumentController from "/assets/controllers/review_document_controller.js"
import RepositoryPickerController from "/assets/controllers/repository_picker_controller.js"

const application = Application.start()
application.debug = false
application.register("diff-viewer", DiffViewerController)
application.register("diagram-viewer", DiagramViewerController)
application.register("review-document", ReviewDocumentController)
application.register("repository-picker", RepositoryPickerController)
