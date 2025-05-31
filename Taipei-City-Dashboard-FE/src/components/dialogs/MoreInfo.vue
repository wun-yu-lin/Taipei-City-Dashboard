<!-- Developed by Taipei Urban Intelligence Center 2023-2024-->

<script setup>
import DashboardComponent from "../../dashboardComponent/DashboardComponent.vue";
import { useDialogStore } from "../../store/dialogStore";
import { useContentStore } from "../../store/contentStore";
import { useAuthStore } from "../../store/authStore";

import DialogContainer from "./DialogContainer.vue";
import HistoryChart from "../charts/HistoryChart.vue";
import DownloadData from "./DownloadData.vue";
import EmbedComponent from "./EmbedComponent.vue";
import { computed, ref } from "vue";
import { useRoute } from "vue-router";
import dayjs from "dayjs";

const dialogStore = useDialogStore();
const contentStore = useContentStore();
const authStore = useAuthStore();
const route = useRoute();
const componentIndex = route.query.index;

const mousePosition = ref({ x: null, y: null });
const showStatisticsTooltip = ref(false);
const showMobileStatisticsTooltip = ref(false);

const tooltipPosition = computed(() => {
	if (!mousePosition.value.x || !mousePosition.value.y) {
		return {
			left: "-1000px",
			top: "-1000px",
		};
	}
	return {
		left: `${mousePosition.value.x - 40}px`,
		top: `${mousePosition.value.y - 100}px`,
	};
});
function updateMouseLocation(e) {
	mousePosition.value.x = e.pageX;
	mousePosition.value.y = e.pageY;
}
function changeShowStatisticsTooltipState(state) {
	showStatisticsTooltip.value = state;
}
function changeShowMobileStatisticsTooltipState(){
	showMobileStatisticsTooltip.value = !showMobileStatisticsTooltip.value
}

function getLinkTag(link, index) {
	if (link.includes("data.taipei")) {
		return `資料集 - ${index + 1} (data.taipei)`;
	} else if (link.includes("data.ntpc")) {
		return `資料集 - ${index + 1} (data.ntpc)`;
	} else if (link.includes("tuic.gov.taipei")) {
		return `大數據中心專案網頁`;
	} else if (link.includes("github.com")) {
		return `GitHub 程式庫`;
	} else {
		return `資料集 - ${index + 1} (其他)`;
	}
}

function handleCloseDialog() {
	const deviceId = authStore.getDeviceID();
	dialogStore.sendComponentViewEvent(deviceId, componentIndex)
	dialogStore.hideAllDialogs()
}
</script>

<template>
  <DialogContainer
    :dialog="`moreInfo`"
    @on-close="handleCloseDialog"
  >
    <div class="moreinfo">
      <DashboardComponent
        :config="dialogStore.moreInfoContent"
        :active-city="dialogStore.moreInfoContent.city"
        :city-tag="contentStore.cityManager.getTagList(dialogStore.moreInfoContent.city)"
        mode="large"
      />
      <div class="moreinfo-info">
        <div class="moreinfo-info-data">
          <h3>
            組件說明（{{
              ` ID: ${dialogStore.moreInfoContent.id}｜Index: ${dialogStore.moreInfoContent.index}｜City: ${dialogStore.moreInfoContent.city}`
            }}）
          </h3>
          <p>{{ dialogStore.moreInfoContent.long_desc }}</p>
          <h3>範例情境</h3>
          <p>{{ dialogStore.moreInfoContent.use_case }}</p>
          <div v-if="dialogStore.moreInfoContent.history_config">
            <h3>歷史軸</h3>
            <h4>*點擊並拉動以檢視細部區間資料</h4>
            <HistoryChart
              :chart_config="
                dialogStore.moreInfoContent.chart_config
              "
              :series="dialogStore.moreInfoContent.history_data"
              :history_config="
                dialogStore.moreInfoContent.history_config
              "
            />
          </div>
          <div v-if="dialogStore.moreInfoContent.links?.length > 0">
            <h3>相關資料</h3>
            <div class="moreinfo-info-links">
              <a
                v-for="(link, index) in dialogStore
                  .moreInfoContent.links"
                :key="link"
                :href="link"
                target="_blank"
                rel="noreferrer"
              >{{ getLinkTag(link, index) }}</a>
            </div>
          </div>
          <div v-if="dialogStore.moreInfoContent.contributors">
            <h3>協作者</h3>
            <div class="moreinfo-info-contributors">
              <div
                v-for="contributor in dialogStore
                  .moreInfoContent.contributors"
                :key="contributor"
              >
                <a
                  :href="
                    contentStore.contributors[contributor]
                      .link
                  "
                  target="_blank"
                  rel="noreferrer"
                ><img
                  :src="
                    contentStore.contributors[
                      contributor
                    ].image.includes('http')
                      ? contentStore.contributors[
                        contributor
                      ].image
                      : `/images/contributors/${contentStore.contributors[contributor].image}`
                  "
                  :alt="`協作者-${contentStore.contributors[contributor].user_name}`"
                >
                </a>
              </div>
            </div>
          </div>
          <div
            v-if="contentStore.currentComponentDynamicInfo"
            class="moreinfo-info-statistics"
            @mouseenter="changeShowStatisticsTooltipState(true)"
            @mousemove="updateMouseLocation"
            @mouseleave="changeShowStatisticsTooltipState(false)" 
          >
            <div class="moreinfo-info-statistics-title">
              <h3>動態資訊</h3>
              <button
                :class="{'hide-button': !authStore.isMobileDevice}"
                @click="changeShowMobileStatisticsTooltipState"
              >
                <span class="icon">info</span>
              </button>
              <div
                v-if="showMobileStatisticsTooltip"
                class="chart-tooltip mobile-tooltip"
              >
                <p>來源：系統日誌分析</p>
                <p>數據計算開始時間：{{ `${dayjs(contentStore.currentComponentDynamicInfo.measured_start).format('YYYY/MM/DD HH:mm:ss')}` }}</p>
                <p>數據計算結束時間：{{ `${dayjs(contentStore.currentComponentDynamicInfo.measured_end).format('YYYY/MM/DD HH:mm:ss')}` }}</p>
              </div>
            </div>
            <div class="moreinfo-info-statistics-content">
              <p><span class="icon">visibility</span>組件點閱人數：{{ `${contentStore.currentComponentDynamicInfo.total_count}` }} 次</p>
              <p><span class="icon">timer</span>平均停留時間：{{ `${Math.round(contentStore.currentComponentDynamicInfo.average_duration_sec)}` }} 秒</p>
            </div>
          </div>
        </div>
        <div class="moreinfo-info-control">
          <button
            v-if="authStore.token"
            @click="
              dialogStore.showReportIssue(
                dialogStore.moreInfoContent.id,
                dialogStore.moreInfoContent.index,
                dialogStore.moreInfoContent.name
              )
            "
          >
            <span>flag</span>回報
          </button>
          <button
            v-if="
              dialogStore.moreInfoContent.chart_config
                .types[0] !== 'MetroChart'
            "
            @click="dialogStore.showDialog('downloadData')"
          >
            <span>download</span>下載
          </button>
          <button @click="dialogStore.showDialog('embedComponent')">
            <span>code</span>內嵌
          </button>
        </div>
        <DownloadData />
        <EmbedComponent />
      </div>
    </div>
  </DialogContainer>
  <Teleport to="body">
    <div
      v-if="showStatisticsTooltip && contentStore.currentComponentDynamicInfo"
      class="chart-tooltip tooltip"
      :style="tooltipPosition"
    >
      <p>來源：系統日誌分析</p>
      <p>數據計算開始時間：{{ `${dayjs(contentStore.currentComponentDynamicInfo.measured_start).format('YYYY/MM/DD HH:mm:ss')}` }}</p>
      <p>數據計算結束時間：{{ `${dayjs(contentStore.currentComponentDynamicInfo.measured_end).format('YYYY/MM/DD HH:mm:ss')}` }}</p>
    </div>
  </Teleport>
</template>

<style scoped lang="scss">
.moreinfo {
	height: fit-content;
	width: 400px;
	display: grid;

	@media (min-width: 820px) {
		width: 720px;
		height: 410px;
		grid-template-columns: 3fr 2fr;
	}

	@media (min-width: 1200px) {
		height: 440px;
		width: 820px;
	}

	@media (min-width: 2200px) {
		height: 550px;
		width: 920px;
	}

	&-info {
		display: flex;
		flex-direction: column;
		padding: var(--font-ms);
		border-top: solid 1px var(--color-border);

		p {
			margin-bottom: 0.75rem;
			color: var(--color-complement-text);
			text-align: justify;
		}

		h4 {
			color: var(--color-complement-text);
			font-weight: 400;
			font-size: 10px;
		}

		@media (min-width: 820px) {
			border-left: solid 1px var(--color-border);
			border-top: none;
		}

		&-data {
			max-height: calc(100% - 2.5rem);
			overflow-y: scroll;
			padding-right: 8px;

			&::-webkit-scrollbar {
				width: 4px;
			}
			&::-webkit-scrollbar-thumb {
				background-color: rgba(136, 135, 135, 0.5);
				border-radius: 4px;
			}
			&::-webkit-scrollbar-thumb:hover {
				background-color: rgba(136, 135, 135, 1);
			}
		}

		&-contributors {
			display: flex;
			flex-wrap: wrap;
			row-gap: 4px;
			column-gap: 4px;
			margin: 4px 0 var(--font-s);

			a {
				display: flex;
				align-items: center;

				p {
					margin: 0;
					transition: color 0.2s;
				}

				img {
					height: var(--font-xl);
					margin-right: 4px;
					border-radius: 50%;
				}

				&:hover p {
					color: var(--color-highlight);
				}
			}
		}

		&-links {
			display: grid;
			grid-template-columns: 1fr 1fr;
			margin: 0 0 var(--font-s);

			a {
				font-size: var(--font-s);
				color: var(--color-complement-text);
				transition: color 0.2s;

				&:hover {
					color: var(--color-highlight);
				}
			}
		}

		&-statistics {
			position: relative;
			overflow: visible;

			&-title {
				display: flex;
				align-items: center;
				gap: 3px;
			}

			h3 {
				margin-bottom: 4px;
			}

			p {
				display: flex;
				align-items: center;
				gap: 4px;
				
				margin-bottom: 1px;
			}

			span {
				font-family: var(--font-icon);
				font-size: var(--font-ms);
			}

			.hide-button {
				display: none;
			}

			.mobile-tooltip {
				position: absolute;
				top: -50px;
				left: 70px;
				z-index: 100;

				p {
					color: var(--color-normal-text);
				}
			}
		}

		&-control {
			display: flex;
			align-items: flex-end;
			justify-content: flex-end;
			flex: 1;

			span {
				margin-right: 4px;
				font-family: var(--font-icon);
				font-size: var(--font-m);
			}

			button {
				display: flex;
				align-items: center;
				margin-left: 8px;
				padding: 2px 4px;
				border-radius: 5px;
				background-color: var(--color-highlight);
				font-size: var(--font-ms);
				transition: opacity 0.2s;

				&:hover {
					opacity: 0.8;
				}
			}
		}
	}
}

.tooltip {
	position: fixed;
	box-shadow: 0px 0px 5px black;
	z-index: 30;
}
</style>
