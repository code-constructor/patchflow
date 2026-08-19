import "/assets/vendor/turbo.js"
import { Application } from "@hotwired/stimulus"
import BlockReferenceController from "/assets/controllers/block_reference_controller.js"
import ChapterNavigationController from "/assets/controllers/chapter_navigation_controller.js"
import CommentThreadController from "/assets/controllers/comment_thread_controller.js"
import DiffViewerController from "/assets/controllers/diff_viewer_controller.js"
import DiagramViewerController from "/assets/controllers/diagram_viewer_controller.js"
import ReviewDocumentController from "/assets/controllers/review_document_controller.js"
import RepositoryPickerController from "/assets/controllers/repository_picker_controller.js"
import ReviewTabsController from "/assets/controllers/review_tabs_controller.js"

const application = Application.start()
application.debug = false
application.register("block-reference", BlockReferenceController)
application.register("chapter-navigation", ChapterNavigationController)
application.register("comment-thread", CommentThreadController)
application.register("diff-viewer", DiffViewerController)
application.register("diagram-viewer", DiagramViewerController)
application.register("review-document", ReviewDocumentController)
application.register("repository-picker", RepositoryPickerController)
application.register("review-tabs", ReviewTabsController)
