import { withFeaturePage } from '../../components/feature-page-gate'
import { useFeatureVisibility } from '../../hooks/use-feature-visibility'
import { usePageShare } from '../../hooks/use-page-share'
import StudentSecondhandContent from './content'

// Preserve the direct/share route; only a page owns WeChat sharing lifecycle.
function StudentSecondhandPage() {
  const visibility = useFeatureVisibility()
  usePageShare({ kind: 'secondhand', enabled: visibility.secondhand })
  return <StudentSecondhandContent />
}

export default withFeaturePage(StudentSecondhandPage)
