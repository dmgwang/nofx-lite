import { motion } from 'framer-motion'
import AnimatedSection from './AnimatedSection'
import { t, Language } from '../../i18n/translations'



interface HowItWorksSectionProps {
    language: Language
}

export default function HowItWorksSection({
    language,
}: HowItWorksSectionProps) {
    return (
        <AnimatedSection id="how-it-works" backgroundColor="var(--brand-dark-gray)">
            <div className="max-w-7xl mx-auto">
                <motion.div
                    className="mt-2 p-6 rounded-xl flex items-start gap-4"
                    style={{
                        background: 'rgba(246, 70, 93, 0.1)',
                        border: '1px solid rgba(246, 70, 93, 0.3)',
                    }}
                    initial={{ opacity: 0, scale: 0.9 }}
                    whileInView={{ opacity: 1, scale: 1 }}
                    viewport={{ once: true }}
                    whileHover={{ scale: 1.02 }}
                >
                    <div
                        className="w-10 h-10 rounded-full flex items-center justify-center flex-shrink-0"
                        style={{ background: 'rgba(246, 70, 93, 0.2)', color: '#F6465D' }}
                    >
                        <svg
                            xmlns="http://www.w3.org/2000/svg"
                            className="w-6 h-6"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            strokeWidth="2"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                        >
                            <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
                            <line x1="12" x2="12" y1="9" y2="13" />
                            <line x1="12" x2="12.01" y1="17" y2="17" />
                        </svg>
                    </div>
                    <div>
                        <div className="font-semibold mb-2" style={{ color: '#F6465D' }}>
                            {t('importantRiskWarning', language)}
                        </div>
                        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                            {t('riskWarningText', language)}
                        </p>
                    </div>
                </motion.div>
            </div>
        </AnimatedSection>
    )
}
