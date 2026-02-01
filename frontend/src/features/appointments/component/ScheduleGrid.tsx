import * as React from "react";
import { addMinutes, format } from "date-fns";
import { Loader2, Plus, Repeat, Trash2 } from "lucide-react";
import type { ScheduleItemModel } from "../../../api/appointments";
import { Button } from "../../../components/ui/button";
import { cn } from "../../../lib/utils";
import {
	clamp,
	formatRange,
	layoutForDay,
	roundTo,
	type TimeRange,
	type VisibleRange,
} from "../utils";

export type ScheduleGridProps = {
	items: ScheduleItemModel[];
	dayStart: Date;
	hours: number[];
	visibleRange: VisibleRange;
	initialLoading: boolean;
	syncing: boolean;
	onQuickCreate: () => void;
	onRangeSelected: (range: TimeRange) => void;
	onAppointmentClick: (appointment: ScheduleItemModel) => void;
};

export function ScheduleGrid({
	items,
	dayStart,
	hours,
	visibleRange,
	initialLoading,
	syncing,
	onQuickCreate,
	onRangeSelected,
	onAppointmentClick,
}: ScheduleGridProps) {
	const blocks = React.useMemo(
		() =>
			layoutForDay(
				items,
				visibleRange.visibleStart,
				visibleRange.visibleEnd,
			),
		[items, visibleRange.visibleEnd, visibleRange.visibleStart],
	);

	const scheduleHeight = hours.length * 64;
	const minuteToPx = scheduleHeight / visibleRange.totalMinutes;

	const scheduleRef = React.useRef<HTMLDivElement | null>(null);
	const [drag, setDrag] = React.useState<{
		startMin: number;
		endMin: number;
	} | null>(null);

	const onGridPointerDown = React.useCallback(
		(e: React.PointerEvent<HTMLDivElement>) => {
			if (initialLoading) return;
			if (!scheduleRef.current) return;
			const target = e.target as HTMLElement | null;
			if (target?.closest("[data-appointment]")) return;

			const rect = scheduleRef.current.getBoundingClientRect();
			const y = e.clientY - rect.top;
			const startMin = clamp(
				roundTo(y / minuteToPx, 15),
				0,
				visibleRange.totalMinutes - 15,
			);
			setDrag({
				startMin,
				endMin: clamp(startMin + 30, 15, visibleRange.totalMinutes),
			});
			scheduleRef.current.setPointerCapture(e.pointerId);
		},
		[initialLoading, minuteToPx, visibleRange.totalMinutes],
	);

	const onGridPointerMove = React.useCallback(
		(e: React.PointerEvent<HTMLDivElement>) => {
			if (initialLoading) return;
			if (!scheduleRef.current || !drag) return;
			const rect = scheduleRef.current.getBoundingClientRect();
			const y = e.clientY - rect.top;
			const raw = clamp(
				roundTo(y / minuteToPx, 15),
				0,
				visibleRange.totalMinutes,
			);

			const a = Math.min(drag.startMin, raw);
			const b = Math.max(drag.startMin, raw);
			const startMin = clamp(a, 0, visibleRange.totalMinutes - 15);
			const endMin = clamp(
				Math.max(b, startMin + 15),
				15,
				visibleRange.totalMinutes,
			);
			setDrag({ startMin, endMin });
		},
		[drag, initialLoading, minuteToPx, visibleRange.totalMinutes],
	);

	const onGridPointerUp = React.useCallback(
		(e: React.PointerEvent<HTMLDivElement>) => {
			if (initialLoading) return;
			if (!scheduleRef.current || !drag) return;
			scheduleRef.current.releasePointerCapture(e.pointerId);
			const start = addMinutes(visibleRange.visibleStart, drag.startMin);
			const end = addMinutes(visibleRange.visibleStart, drag.endMin);
			setDrag(null);
			onRangeSelected({ start, end });
		},
		[drag, initialLoading, onRangeSelected, visibleRange.visibleStart],
	);

	return (
		<div className="grid grid-cols-[76px_1fr]">
			<div className="border-r border-border bg-muted/30">
				<div className="h-10" />
				{hours.map((h) => (
					<div
						key={h}
						className="relative h-16 px-3 text-right text-xs font-medium text-muted-foreground"
					>
						<div className="absolute right-3 top-[-0.45rem]">
							{(() => {
								const d = new Date(dayStart);
								d.setHours(h, 0, 0, 0);
								return format(d, "h a");
							})()}
						</div>
					</div>
				))}
			</div>

			<div className="relative">
				<div className="h-10 px-4 py-2 text-xs text-muted-foreground">
					Local time
				</div>

				<div
					ref={scheduleRef}
					className="relative select-none px-3 pb-3"
					onPointerDown={onGridPointerDown}
					onPointerMove={onGridPointerMove}
					onPointerUp={onGridPointerUp}
					style={{ height: scheduleHeight }}
				>
					{initialLoading ? (
						<div className="absolute inset-x-3 top-0 z-10 grid h-full place-items-center rounded-lg bg-background/70 backdrop-blur-[1px]">
							<div className="flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-2 shadow-sm">
								<Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
								<div className="text-sm font-medium">
									Loading appointments…
								</div>
							</div>
						</div>
					) : null}
					<div
						className="absolute inset-x-3 top-0 rounded-lg border border-border/80 bg-[linear-gradient(to_bottom,transparent_0px,transparent_63px,hsl(var(--border))_64px)] bg-size-[100%_64px] shadow-[inset_0_1px_0_rgba(255,255,255,0.35)]"
						style={{ height: scheduleHeight }}
					/>

					<div className="absolute inset-x-3 top-0 h-full">
						{blocks.map((b) => {
							const top = b.startMin * minuteToPx;
							const height = Math.max(
								18,
								(b.endMin - b.startMin) * minuteToPx,
							);
							const widthPct = 100 / b.columnCount;
							const leftPct = b.column * widthPct;
							const start = b.item.startTime;
							const end = b.item.endTime;
							const isRecurring = b.item.kind === "occurrence";

							return (
								<button
									key={`${b.item.kind}:${b.item.id}`}
									type="button"
									data-appointment
									onClick={() => {
										if (b.item.kind === "appointment") {
											onAppointmentClick(b.item);
										}
									}}
									className={cn(
										"group absolute rounded-lg border p-2 text-left shadow-sm ring-1 ring-black/5 transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
										isRecurring
											? "border-border/70 bg-[linear-gradient(135deg,color-mix(in_hsl,hsl(var(--accent))_18%,hsl(var(--card)))_0%,hsl(var(--card))_58%,color-mix(in_hsl,hsl(var(--secondary))_26%,hsl(var(--card)))_100%)] hover:-translate-y-0.5 hover:shadow-md"
											: "border-border/70 bg-[linear-gradient(135deg,color-mix(in_hsl,hsl(var(--primary))_10%,hsl(var(--card)))_0%,hsl(var(--card))_55%,color-mix(in_hsl,hsl(var(--accent))_10%,hsl(var(--card)))_100%)] hover:-translate-y-0.5 hover:shadow-md",
									)}
									style={{
										top,
										height,
										left: `${leftPct}%`,
										width: `${widthPct}%`,
									}}
								>
									<div className="flex items-start justify-between gap-2">
										<div className="min-w-0">
											<div className="truncate text-sm font-semibold tracking-tight">
												{b.item.title}
											</div>
											<div className="mt-0.5 truncate text-[0.72rem] text-muted-foreground">
												{formatRange(start, end)}
											</div>
										</div>
										{isRecurring ? (
											<div className="mt-0.5 inline-flex items-center gap-1 rounded-md border border-border/70 bg-background/60 px-1.5 py-1 text-[0.68rem] font-medium text-muted-foreground">
												<Repeat className="h-3.5 w-3.5" />
												<span className="leading-none">
													Recurring
												</span>
											</div>
										) : (
											<div className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-md bg-background/60 text-muted-foreground opacity-0 transition group-hover:opacity-100">
												<Trash2 className="h-3.5 w-3.5" />
											</div>
										)}
									</div>
									{b.item.notes.trim().length ? (
										<div className="mt-1 truncate text-[0.72rem] text-muted-foreground">
											{b.item.notes}
										</div>
									) : null}
								</button>
							);
						})}

						{drag ? (
							<div
								className="absolute rounded-lg border border-primary/35 bg-primary/10 ring-1 ring-primary/20"
								style={{
									top: drag.startMin * minuteToPx,
									height:
										(drag.endMin - drag.startMin) *
										minuteToPx,
									left: 0,
									right: 0,
								}}
							/>
						) : null}
					</div>

					{!initialLoading && !syncing && items.length === 0 ? (
						<div className="absolute inset-x-3 top-14 grid place-items-center rounded-lg border border-dashed border-border bg-muted/20 p-6">
							<div className="max-w-sm text-center">
								<div className="text-sm font-semibold tracking-tight">
									No appointments today
								</div>
								<div className="mt-1 text-xs text-muted-foreground">
									Drag across the schedule to create one, or
									use New.
								</div>
								<div className="mt-3">
									<Button
										variant="secondary"
										className="gap-2"
										onClick={onQuickCreate}
									>
										<Plus className="h-4 w-4" />
										Create appointment
									</Button>
								</div>
							</div>
						</div>
					) : null}
				</div>
			</div>
		</div>
	);
}
