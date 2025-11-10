import AnimatedSection from './AnimatedSection'
import { CryptoFeatureCard } from '../CryptoFeatureCard'
import { Code, Cpu, Lock } from 'lucide-react'
import { t, Language } from '../../i18n/translations'

interface FeaturesSectionProps {
    language: Language
}

export default function FeaturesSection({ language }: FeaturesSectionProps) {
    return (
        <AnimatedSection id="features">
            <div className="max-w-7xl mx-auto">

                <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-8 max-w-7xl mx-auto">
                    <CryptoFeatureCard
                        icon={<Code className="w-8 h-8" />}
                        title={t('openSourceSelfHosted', language)}
                        description={t('openSourceDesc', language)}
                        features={[
                            t('openSourceFeatures1', language),
                            t('openSourceFeatures2', language),
                            t('openSourceFeatures3', language),
                            t('openSourceFeatures4', language),
                        ]}
                        delay={0}
                    />
                    <CryptoFeatureCard
                        icon={<Cpu className="w-8 h-8" />}
                        title={t('multiAgentCompetition', language)}
                        description={t('multiAgentDesc', language)}
                        features={[
                            t('multiAgentFeatures1', language),
                            t('multiAgentFeatures2', language),
                            t('multiAgentFeatures3', language),
                            t('multiAgentFeatures4', language),
                        ]}
                        delay={0.1}
                    />
                    <CryptoFeatureCard
                        icon={<Lock className="w-8 h-8" />}
                        title={t('secureReliableTrading', language)}
                        description={t('secureDesc', language)}
                        features={[
                            t('secureFeatures1', language),
                            t('secureFeatures2', language),
                            t('secureFeatures3', language),
                            t('secureFeatures4', language),
                        ]}
                        delay={0.2}
                    />
                </div>
            </div>
        </AnimatedSection>
    )
}
