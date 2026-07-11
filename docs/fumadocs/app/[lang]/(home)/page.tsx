"use client";

import Link from 'next/link';
import { motion, useReducedMotion } from 'framer-motion';
import { Terminal, Activity, FileText, Command, ArrowUpRight } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';

const customBezier: [number, number, number, number] = [0.32, 0.72, 0, 1];
const transitionPhysics = { duration: 1.2, ease: customBezier };

const translations = {
  en: {
    heroTitle1: "Terminal control",
    heroTitle2: "for mihomo.",
    heroSubtitle: "Manage proxy nodes, monitor live traffic, and stream logs with uncompromised speed directly from the command line.",
    initSetup: "Initialize Setup",
    viewSource: "View Source Code",
    bento1Title: "Node Orchestration.",
    bento1Desc: "Seamlessly switch proxy nodes and execute batch latency tests across groups with zero UI overhead. Pure terminal flow.",
    bento2Title: "Live Telemetry",
    bento2Desc: "Monitor traffic and manage active connections with real-time feedback loops.",
    bento3Title: "Log Streaming",
    bento3Desc: "Stream and filter logs instantaneously with integrated inline search.",
    bento4Title: "Haptic Control.",
    bento4Desc: "Designed entirely for speed. Navigate with tactile keyboard shortcuts without breaking your workflow momentum. Never touch the mouse.",
    footerTitle: "Deploy Mihosh.",
    footerBtn: "Read Documentation"
  },
  zh: {
    heroTitle1: "终端掌控",
    heroTitle2: "驱动 mihomo",
    heroSubtitle: "在命令行中直接管理代理节点、实时监控网络流量，并以极速查看日志流。",
    initSetup: "快速开始",
    viewSource: "查看源码",
    bento1Title: "节点调度",
    bento1Desc: "零额外 UI 开销，在终端流中无缝切换代理节点，跨代理组执行批量延迟测试。",
    bento2Title: "实时遥测",
    bento2Desc: "通过实时的反馈循环监控流量，管理活跃连接。",
    bento3Title: "日志流",
    bento3Desc: "即时流式传输并过滤日志，内置行内搜索。",
    bento4Title: "触觉掌控",
    bento4Desc: "专为速度而生。使用手感极佳的快捷键导航，不打断工作流。摆脱鼠标依赖。",
    footerTitle: "部署 Mihosh",
    footerBtn: "阅读文档"
  }
};

export default function HomePage() {
  const reduceMotion = useReducedMotion();
  const [isMounted, setIsMounted] = useState(false);
  const params = useParams();
  const lang = (params?.lang as string) || 'zh';
  const t = translations[lang as keyof typeof translations] || translations.zh;

  useEffect(() => {
    setIsMounted(true);
  }, []);

  // Standard entry animation
  const entryAnim = reduceMotion
    ? {}
    : {
        initial: { opacity: 0, y: 48, filter: 'blur(12px)' },
        whileInView: { opacity: 1, y: 0, filter: 'blur(0px)' },
        viewport: { once: true, margin: "-50px" },
      };

  return (
    <main className="w-full bg-[#FDFCFB] dark:bg-[#050505] text-neutral-900 dark:text-neutral-50 font-sans min-h-[100dvh] relative overflow-hidden selection:bg-neutral-900 selection:text-white dark:selection:bg-white dark:selection:text-neutral-900">
       
       {/* Ambient Ethereal Glow Background (No blur on scrolling container, just large soft gradients) */}
       <div className="pointer-events-none absolute inset-0 z-0 overflow-hidden">
         <div className="absolute top-[-20%] left-[-10%] w-[60%] h-[60%] rounded-full bg-indigo-500/[0.04] dark:bg-indigo-500/[0.12] blur-[140px] mix-blend-screen" />
         <div className="absolute bottom-[-10%] right-[-10%] w-[50%] h-[60%] rounded-full bg-emerald-500/[0.04] dark:bg-emerald-500/[0.08] blur-[140px] mix-blend-screen" />
       </div>

       {/* Hero Section */}
       <section className="relative z-10 min-h-[85vh] flex flex-col justify-center px-4 md:px-12 pt-24 pb-32 md:pt-32 md:pb-48 max-w-[90rem] mx-auto w-full">
         <motion.div
           initial={reduceMotion ? false : { opacity: 0, y: 64, filter: 'blur(12px)' }}
           animate={isMounted ? { opacity: 1, y: 0, filter: 'blur(0px)' } : {}}
           transition={{ ...transitionPhysics, duration: 1.4 }}
           className="flex flex-col gap-10 max-w-5xl -mt-12 md:-mt-20"
         >
           
           <h1 className="text-[clamp(3.5rem,9vw,8.5rem)] leading-[0.98] font-semibold tracking-[-0.04em] text-neutral-900 dark:text-[#FAFAFA]">
             {t.heroTitle1} <br />
             <span className="text-neutral-400 dark:text-[#333333]">{t.heroTitle2}</span>
           </h1>
           
           <p className="text-xl md:text-2xl text-neutral-500 dark:text-neutral-500 max-w-[38ch] leading-[1.6] font-medium mt-2">
             {t.heroSubtitle}
           </p>
           
           <div className="flex flex-col sm:flex-row gap-8 mt-12 items-start sm:items-center">
             {/* Nested CTA (Button-in-Button) */}
             <Link
               href={`/${lang}/docs`}
               className="group relative inline-flex items-center gap-6 rounded-full bg-neutral-900 dark:bg-white text-white dark:text-neutral-900 pl-8 pr-2.5 py-2.5 font-medium transition-transform duration-700 ease-[cubic-bezier(0.32,0.72,0,1)] active:scale-[0.97]"
             >
               <span className="text-[15px] tracking-wide">{t.initSetup}</span>
               <div className="flex h-11 w-11 items-center justify-center rounded-full bg-white/20 dark:bg-black/10 transition-transform duration-700 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:translate-x-1 group-hover:-translate-y-[1px] group-hover:scale-105">
                 <ArrowUpRight className="h-5 w-5" strokeWidth={1.5} />
               </div>
             </Link>
             
             <a
               href="https://github.com/AimAI-Labs/mihosh"
               target="_blank"
               rel="noreferrer"
               className="group relative inline-flex items-center gap-3 text-[15px] font-medium text-neutral-500 dark:text-neutral-400 transition-colors duration-700 hover:text-neutral-900 dark:hover:text-white"
             >
               {t.viewSource}
               <div className="absolute -bottom-2 left-0 h-[1px] w-0 bg-current transition-all duration-700 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:w-full" />
             </a>
           </div>
         </motion.div>
       </section>

       {/* Asymmetrical Bento Grid */}
       <section className="relative z-10 px-4 md:px-12 pb-40 md:pb-56 max-w-[90rem] mx-auto w-full">
         <motion.div 
            {...entryAnim}
            transition={{ ...transitionPhysics, duration: 1.2 }}
            className="grid grid-cols-1 md:grid-cols-12 gap-6"
         >
           {/* Primary Bento - Double Bezel */}
           <div className="md:col-span-8 md:row-span-2 group relative p-2 md:p-2.5 rounded-[2.5rem] bg-black/[0.02] dark:bg-white/[0.02] ring-1 ring-black/5 dark:ring-white/10 transition-all duration-1000 ease-[cubic-bezier(0.32,0.72,0,1)] hover:ring-2 hover:ring-black/15 dark:hover:ring-white/20">
             <div className="relative h-full w-full rounded-[calc(2.5rem-0.5rem)] bg-white dark:bg-[#0A0A0A] shadow-[inset_0_1px_1px_rgba(0,0,0,0.03)] dark:shadow-[inset_0_1px_1px_rgba(255,255,255,0.03)] p-10 md:p-16 flex flex-col justify-between overflow-hidden min-h-[450px]">
                <div className="absolute top-0 right-0 w-full h-[80%] bg-gradient-to-bl from-indigo-500/5 to-transparent blur-3xl pointer-events-none" />
                <Terminal className="w-12 h-12 mb-16 text-neutral-900 dark:text-white opacity-80 transition-transform duration-1000 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:scale-125 group-hover:-rotate-12" strokeWidth={1} />
                <div className="relative z-10 max-w-xl">
                  <h3 className="text-3xl md:text-5xl font-semibold tracking-tight mb-5">{t.bento1Title}</h3>
                  <p className="text-neutral-500 dark:text-neutral-500 text-lg leading-relaxed">
                    {t.bento1Desc}
                  </p>
                </div>
             </div>
           </div>

           {/* Secondary Bento 1 */}
           <div className="md:col-span-4 md:row-span-1 group relative p-2 md:p-2.5 rounded-[2.5rem] bg-black/[0.02] dark:bg-white/[0.02] ring-1 ring-black/5 dark:ring-white/10 transition-all duration-1000 ease-[cubic-bezier(0.32,0.72,0,1)] hover:ring-2 hover:ring-black/15 dark:hover:ring-white/20">
             <div className="relative h-full w-full rounded-[calc(2.5rem-0.5rem)] bg-[#FAFAFA] dark:bg-[#0A0A0A] shadow-[inset_0_1px_1px_rgba(0,0,0,0.03)] dark:shadow-[inset_0_1px_1px_rgba(255,255,255,0.03)] p-10 md:p-12 flex flex-col justify-between min-h-[320px]">
                <Activity className="w-10 h-10 mb-10 text-neutral-900 dark:text-white opacity-80 transition-transform duration-1000 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:scale-125 group-hover:rotate-12" strokeWidth={1} />
                <div>
                  <h3 className="text-2xl font-medium tracking-tight mb-3">{t.bento2Title}</h3>
                  <p className="text-neutral-500 dark:text-neutral-500 leading-relaxed text-[15px]">
                    {t.bento2Desc}
                  </p>
                </div>
             </div>
           </div>

           {/* Secondary Bento 2 */}
           <div className="md:col-span-4 md:row-span-1 group relative p-2 md:p-2.5 rounded-[2.5rem] bg-black/[0.02] dark:bg-white/[0.02] ring-1 ring-black/5 dark:ring-white/10 transition-all duration-1000 ease-[cubic-bezier(0.32,0.72,0,1)] hover:ring-2 hover:ring-black/15 dark:hover:ring-white/20">
             <div className="relative h-full w-full rounded-[calc(2.5rem-0.5rem)] bg-[#FAFAFA] dark:bg-[#0A0A0A] shadow-[inset_0_1px_1px_rgba(0,0,0,0.03)] dark:shadow-[inset_0_1px_1px_rgba(255,255,255,0.03)] p-10 md:p-12 flex flex-col justify-between min-h-[320px]">
                <FileText className="w-10 h-10 mb-10 text-neutral-900 dark:text-white opacity-80 transition-transform duration-1000 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:scale-125 group-hover:-rotate-12" strokeWidth={1} />
                <div>
                  <h3 className="text-2xl font-medium tracking-tight mb-3">{t.bento3Title}</h3>
                  <p className="text-neutral-500 dark:text-neutral-500 leading-relaxed text-[15px]">
                    {t.bento3Desc}
                  </p>
                </div>
             </div>
           </div>

           {/* Full Width Keyboard Driven */}
           <div className="md:col-span-12 md:row-span-1 group relative p-2 md:p-2.5 rounded-[2.5rem] bg-black/[0.02] dark:bg-white/[0.02] ring-1 ring-black/5 dark:ring-white/10 transition-all duration-1000 ease-[cubic-bezier(0.32,0.72,0,1)] hover:ring-2 hover:ring-black/15 dark:hover:ring-white/20 mt-6 md:mt-0">
             <div className="relative h-full w-full rounded-[calc(2.5rem-0.5rem)] bg-white dark:bg-[#0A0A0A] shadow-[inset_0_1px_1px_rgba(0,0,0,0.03)] dark:shadow-[inset_0_1px_1px_rgba(255,255,255,0.03)] p-10 md:p-20 flex flex-col md:flex-row md:items-center justify-between gap-16 overflow-hidden">
                <div className="absolute -bottom-[50%] -left-[10%] w-[60%] h-[150%] bg-gradient-to-tr from-emerald-500/5 to-transparent blur-3xl pointer-events-none" />
                <div className="max-w-2xl relative z-10">
                  <Command className="w-12 h-12 mb-12 text-neutral-900 dark:text-white opacity-80 transition-transform duration-1000 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:scale-125 group-hover:rotate-12" strokeWidth={1} />
                  <h3 className="text-3xl md:text-5xl font-semibold tracking-tight mb-5">{t.bento4Title}</h3>
                  <p className="text-neutral-500 dark:text-neutral-500 text-lg leading-relaxed">
                    {t.bento4Desc}
                  </p>
                </div>
                
                <div className="flex flex-col gap-5 relative z-10 opacity-90 transition-transform duration-1000 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:scale-[1.02]">
                  <div className="flex gap-4 justify-end">
                    <kbd className="w-16 h-16 flex items-center justify-center rounded-[1.25rem] bg-[#F7F7F7] dark:bg-neutral-900/40 border border-black/5 dark:border-white/5 font-mono text-2xl text-neutral-700 dark:text-neutral-300 shadow-[inset_0_1px_2px_rgba(255,255,255,0.8),0_4px_12px_rgba(0,0,0,0.03)] dark:shadow-[inset_0_1px_2px_rgba(255,255,255,0.03),0_4px_12px_rgba(0,0,0,0.2)]">J</kbd>
                    <kbd className="w-16 h-16 flex items-center justify-center rounded-[1.25rem] bg-[#F7F7F7] dark:bg-neutral-900/40 border border-black/5 dark:border-white/5 font-mono text-2xl text-neutral-700 dark:text-neutral-300 shadow-[inset_0_1px_2px_rgba(255,255,255,0.8),0_4px_12px_rgba(0,0,0,0.03)] dark:shadow-[inset_0_1px_2px_rgba(255,255,255,0.03),0_4px_12px_rgba(0,0,0,0.2)]">K</kbd>
                  </div>
                  <div className="flex gap-4 justify-end">
                    <kbd className="w-36 h-16 flex items-center justify-center rounded-[1.25rem] bg-[#F7F7F7] dark:bg-neutral-900/40 border border-black/5 dark:border-white/5 font-mono text-sm tracking-[0.2em] uppercase text-neutral-700 dark:text-neutral-300 shadow-[inset_0_1px_2px_rgba(255,255,255,0.8),0_4px_12px_rgba(0,0,0,0.03)] dark:shadow-[inset_0_1px_2px_rgba(255,255,255,0.03),0_4px_12px_rgba(0,0,0,0.2)]">Enter</kbd>
                  </div>
                </div>
             </div>
           </div>
         </motion.div>
       </section>

       {/* Footer CTA */}
       <section className="relative py-40 md:py-64 px-4 flex flex-col items-center text-center">
         <motion.div
           {...entryAnim}
           transition={{ ...transitionPhysics, duration: 1.2 }}
           className="relative z-10 flex flex-col items-center"
         >
           <h2 className="text-[clamp(3.5rem,7vw,6.5rem)] font-semibold tracking-[-0.04em] mb-14 text-neutral-900 dark:text-white leading-none">
             {t.footerTitle}
           </h2>
           <Link
             href={`/${lang}/docs`}
             className="group relative inline-flex items-center gap-5 rounded-full bg-neutral-900 dark:bg-white text-white dark:text-neutral-900 pl-10 pr-3 py-3 font-medium transition-transform duration-700 ease-[cubic-bezier(0.32,0.72,0,1)] active:scale-[0.97]"
           >
             <span className="text-[15px] tracking-wide">{t.footerBtn}</span>
             <div className="flex h-12 w-12 items-center justify-center rounded-full bg-white/20 dark:bg-black/10 transition-transform duration-700 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:translate-x-1 group-hover:-translate-y-[1px] group-hover:scale-105">
               <ArrowUpRight className="h-5 w-5" strokeWidth={1.5} />
             </div>
           </Link>
         </motion.div>
       </section>

    </main>
  );
}
